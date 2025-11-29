import json
import uuid
import os
import asyncio
import aiohttp
import websockets
from fastapi import FastAPI, UploadFile, File, Form
from fastapi.responses import FileResponse
from pathlib import Path

# ============================================================
# FastAPI Application
# ============================================================

app = FastAPI()

COMFYUI_URL = "http://localhost:8000"
STORAGE_PATH = "/home/v0idhrt/Документы/govno/shyyyy/source"
WORKFLOW_PATH = "workflow.json"

# ============================================================
# Helper Functions
# ============================================================

async def upload_image_to_comfyui(image_bytes: bytes, filename: str) -> dict:
    """Загрузка изображения в ComfyUI"""
    async with aiohttp.ClientSession() as session:
        data = aiohttp.FormData()
        data.add_field('image', image_bytes, filename=filename)

        async with session.post(f"{COMFYUI_URL}/upload/image", data=data) as resp:
            result = await resp.json()
            print(f"[COMFYUI] Upload response: {result}")
            return result

async def queue_workflow(workflow: dict) -> str:
    """Постановка workflow в очередь ComfyUI"""
    async with aiohttp.ClientSession() as session:
        payload = {"prompt": workflow}
        async with session.post(f"{COMFYUI_URL}/prompt", json=payload) as resp:
            result = await resp.json()
            return result['prompt_id']

async def listen_websocket(prompt_id: str, client_id: str, completed_event: asyncio.Event):
    """Слушаем WebSocket для прогресса и завершения с автоматическим переподключением"""
    uri = f"ws://localhost:8000/ws?clientId={client_id}"

    node_1239_executed = False
    max_retries = 10
    retry_delay = 2

    for attempt in range(max_retries):
        try:
            print(f"[COMFYUI] Connecting to WebSocket (attempt {attempt + 1}/{max_retries}): {uri}")

            async with websockets.connect(uri, open_timeout=300, close_timeout=10, ping_interval=None) as ws:
                print(f"[COMFYUI] WebSocket connected successfully")

                while True:
                    try:
                        message = await asyncio.wait_for(ws.recv(), timeout=300)
                        data = json.loads(message)

                        msg_type = data.get('type')

                        # Логируем все события для дебага
                        print(f"[COMFYUI] WS event: {msg_type} | data: {data.get('data', {})}")

                        if msg_type == 'execution_start':
                            ws_prompt_id = data.get('data', {}).get('prompt_id')
                            if ws_prompt_id == prompt_id:
                                print(f"[COMFYUI] Workflow execution started!")

                        if msg_type == 'executing':
                            node = data.get('data', {}).get('node')
                            ws_prompt_id = data.get('data', {}).get('prompt_id')

                            if ws_prompt_id == prompt_id:
                                # Отслеживаем выполнение ноды 1239 (Save Image)
                                if node == '1239':
                                    node_1239_executed = True
                                    print(f"[COMFYUI] Node 1239 (Save Image) started executing")

                                # Завершаем только если нода 1239 уже выполнялась и пришёл node=null
                                if node is None and node_1239_executed:
                                    print(f"[COMFYUI] Workflow completed! (node 1239 finished, workflow ended)")
                                    completed_event.set()
                                    return
                                elif node:
                                    print(f"[COMFYUI] Executing node: {node}")

                        if msg_type == 'progress':
                            value = data.get('data', {}).get('value', 0)
                            max_val = data.get('data', {}).get('max', 100)
                            node = data.get('data', {}).get('node', '?')
                            print(f"[COMFYUI] Progress [{node}]: {value}/{max_val}")

                    except asyncio.TimeoutError:
                        print(f"[COMFYUI] WebSocket timeout - workflow took too long")
                        raise TimeoutError("Workflow execution timeout")
                    except websockets.exceptions.ConnectionClosed as e:
                        print(f"[COMFYUI] WebSocket connection closed: {e}")
                        # Переподключаемся
                        break
                    except Exception as e:
                        print(f"[COMFYUI] WebSocket message error: {e}")
                        break

        except asyncio.TimeoutError:
            print(f"[COMFYUI] WebSocket handshake timeout on attempt {attempt + 1}")
            if attempt < max_retries - 1:
                print(f"[COMFYUI] Retrying in {retry_delay} seconds...")
                await asyncio.sleep(retry_delay)
                continue
            else:
                raise TimeoutError("WebSocket handshake failed after all retries")
        except Exception as e:
            print(f"[COMFYUI] WebSocket connection error on attempt {attempt + 1}: {e}")
            if attempt < max_retries - 1:
                print(f"[COMFYUI] Retrying in {retry_delay} seconds...")
                await asyncio.sleep(retry_delay)
                continue
            else:
                raise Exception(f"WebSocket connection failed after {max_retries} attempts: {e}")

async def poll_history(prompt_id: str):
    """Polling /history каждую секунду"""
    async with aiohttp.ClientSession() as session:
        max_attempts = 300

        for attempt in range(max_attempts):
            # Проверяем сразу, без задержки в первый раз
            if attempt > 0:
                await asyncio.sleep(1)

            try:
                async with session.get(f"{COMFYUI_URL}/history/{prompt_id}") as resp:
                    if resp.status == 200:
                        history = await resp.json()

                        if prompt_id in history:
                            prompt_data = history[prompt_id]

                            # Проверяем наличие outputs
                            if 'outputs' in prompt_data and '1239' in prompt_data['outputs']:
                                print(f"[COMFYUI] Workflow completed! (detected via polling)")
                                return True

                            # Проверяем ошибки
                            if prompt_data.get('status', {}).get('status_str') == 'error':
                                error_msg = prompt_data.get('status', {}).get('messages', [])
                                raise Exception(f"ComfyUI error: {error_msg}")

            except Exception as e:
                print(f"[COMFYUI] Polling error: {e}")
                continue

        raise TimeoutError(f"Workflow did not complete in {max_attempts} seconds")

async def wait_for_completion(prompt_id: str, client_id: str):
    """Ожидание завершения через WebSocket + polling как backup"""
    completed_event = asyncio.Event()

    # Запускаем WebSocket и polling параллельно
    # Завершаемся когда любая из задач успешно завершится
    ws_task = asyncio.create_task(listen_websocket(prompt_id, client_id, completed_event))
    poll_task = asyncio.create_task(poll_history(prompt_id))

    done, pending = await asyncio.wait(
        [ws_task, poll_task],
        return_when=asyncio.FIRST_COMPLETED
    )

    # Отменяем незавершенные задачи
    for task in pending:
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass

    # Проверяем результат завершенной задачи
    for task in done:
        try:
            result = task.result()
            print(f"[COMFYUI] Completion detected by: {'WebSocket' if task == ws_task else 'Polling'}")
            return result
        except Exception as e:
            # Если одна задача упала, проверяем другую
            print(f"[COMFYUI] Task failed: {e}")
            continue

    # Если обе упали, пробрасываем исключение
    raise Exception("Both WebSocket and polling failed")

async def get_result_image(prompt_id: str) -> tuple:
    """Получение результата из history"""
    async with aiohttp.ClientSession() as session:
        # Retry до 10 раз с задержкой 0.5 секунды
        for attempt in range(10):
            async with session.get(f"{COMFYUI_URL}/history/{prompt_id}") as resp:
                history = await resp.json()

                if prompt_id in history:
                    print(f"[COMFYUI] History found on attempt {attempt + 1}")
                    prompt_data = history[prompt_id]

                    if 'outputs' not in prompt_data:
                        raise Exception(f"No outputs in history for prompt {prompt_id}")

                    if '1239' not in prompt_data['outputs']:
                        print(f"[COMFYUI] Available outputs: {list(prompt_data['outputs'].keys())}")
                        raise Exception(f"Node 1239 not found in outputs")

                    outputs = prompt_data['outputs']['1239']
                    print(f"[COMFYUI] Node 1239 outputs: {outputs}")

                    filename = outputs['images'][0]['filename']

                    # Скачиваем изображение
                    async with session.get(f"{COMFYUI_URL}/view?filename={filename}&type=output") as img_resp:
                        return filename, await img_resp.read()
                else:
                    print(f"[COMFYUI] History not ready yet, attempt {attempt + 1}/10")
                    await asyncio.sleep(0.5)

        raise Exception(f"History for prompt {prompt_id} not found after 10 attempts")

# ============================================================
# API Endpoints
# ============================================================

@app.post("/process")
async def process_workflow(
    gray_image: UploadFile = File(...),
    reference_image: UploadFile = File(...),
    user_id: str = Form(...)
):
    """Обработка изображений через ComfyUI"""

    print(f"[COMFYUI] Starting workflow for user {user_id}")

    # 1. Загрузка изображений в ComfyUI
    gray_bytes = await gray_image.read()
    ref_bytes = await reference_image.read()

    # Добавляем UUID к именам файлов для избежания кеширования
    unique_id = str(uuid.uuid4())[:8]
    gray_filename = f"{unique_id}_{gray_image.filename}"
    ref_filename = f"{unique_id}_{reference_image.filename}"

    print(f"[COMFYUI] Uploading gray image: {gray_filename}")
    gray_result = await upload_image_to_comfyui(gray_bytes, gray_filename)
    gray_name = gray_result['name']
    print(f"[COMFYUI] Gray image uploaded as: {gray_name}")

    print(f"[COMFYUI] Uploading reference image: {ref_filename}")
    ref_result = await upload_image_to_comfyui(ref_bytes, ref_filename)
    ref_name = ref_result['name']
    print(f"[COMFYUI] Reference image uploaded as: {ref_name}")

    # 2. Модификация workflow
    with open(WORKFLOW_PATH) as f:
        workflow = json.load(f)

    print(f"[COMFYUI] Original workflow node 1222: {workflow['1222']}")
    print(f"[COMFYUI] Original workflow node 1231: {workflow['1231']}")

    workflow["1222"]["inputs"]["image"] = gray_name
    workflow["1231"]["inputs"]["image"] = ref_name

    print(f"[COMFYUI] Modified workflow node 1222: {workflow['1222']}")
    print(f"[COMFYUI] Modified workflow node 1231: {workflow['1231']}")

    # 3. Постановка в очередь
    print(f"[COMFYUI] Queueing workflow...")
    prompt_id = await queue_workflow(workflow)
    client_id = str(uuid.uuid4())
    print(f"[COMFYUI] Workflow queued with prompt_id: {prompt_id}")

    # 4. Ожидание завершения
    print(f"[COMFYUI] Waiting for completion...")
    await wait_for_completion(prompt_id, client_id)

    # 5. Получение результата
    print(f"[COMFYUI] Downloading result...")
    result_filename, result_bytes = await get_result_image(prompt_id)
    print(f"[COMFYUI] Result downloaded: {result_filename}")

    # 6. Сохранение локально
    save_dir = Path(STORAGE_PATH) / user_id / "comfyui" / "results"
    save_dir.mkdir(parents=True, exist_ok=True)

    save_path = save_dir / f"{prompt_id}_result.png"
    with open(save_path, 'wb') as f:
        f.write(result_bytes)

    print(f"[COMFYUI] Saved to: {save_path}")
    print(f"[COMFYUI] Workflow complete!")

    # 7. Возврат ответа
    return {
        "prompt_id": prompt_id,
        "result_url": f"/download/{user_id}/{prompt_id}_result.png"
    }

@app.get("/download/{user_id}/{filename}")
async def download_result(user_id: str, filename: str):
    """Скачивание результата"""
    file_path = Path(STORAGE_PATH) / user_id / "comfyui" / "results" / filename
    return FileResponse(file_path)
