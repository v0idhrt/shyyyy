import json
import uuid
import os
import asyncio
import aiohttp
import websockets
from fastapi import FastAPI, UploadFile, File, Form, HTTPException
from fastapi.responses import FileResponse, StreamingResponse
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

async def listen_websocket(prompt_id: str, client_id: str, completed_event: asyncio.Event, progress_queue: asyncio.Queue = None):
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
                                if progress_queue:
                                    await progress_queue.put({"type": "execution_start", "message": "Workflow execution started"})

                        if msg_type == 'executing':
                            node = data.get('data', {}).get('node')
                            ws_prompt_id = data.get('data', {}).get('prompt_id')

                            if ws_prompt_id == prompt_id:
                                # Отслеживаем выполнение ноды 1239 (Save Image)
                                if node == '1239':
                                    node_1239_executed = True
                                    print(f"[COMFYUI] Node 1239 (Save Image) started executing")
                                    if progress_queue:
                                        await progress_queue.put({"type": "node_executing", "node": node, "message": "Saving image"})

                                # Завершаем только если нода 1239 уже выполнялась и пришёл node=null
                                if node is None and node_1239_executed:
                                    print(f"[COMFYUI] Workflow completed! (node 1239 finished, workflow ended)")
                                    completed_event.set()
                                    return
                                elif node:
                                    print(f"[COMFYUI] Executing node: {node}")
                                    if progress_queue:
                                        await progress_queue.put({"type": "node_executing", "node": node, "message": f"Executing node {node}"})

                        if msg_type == 'progress':
                            value = data.get('data', {}).get('value', 0)
                            max_val = data.get('data', {}).get('max', 100)
                            node = data.get('data', {}).get('node', '?')
                            print(f"[COMFYUI] Progress [{node}]: {value}/{max_val}")
                            if progress_queue:
                                await progress_queue.put({
                                    "type": "progress",
                                    "node": node,
                                    "value": value,
                                    "max": max_val,
                                    "percent": int((value / max_val) * 100) if max_val > 0 else 0
                                })

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

async def wait_for_completion(prompt_id: str, client_id: str, progress_queue: asyncio.Queue = None):
    """Ожидание завершения через WebSocket + polling как backup"""
    completed_event = asyncio.Event()

    # Запускаем WebSocket и polling параллельно
    # Завершаемся когда любая из задач успешно завершится
    ws_task = asyncio.create_task(listen_websocket(prompt_id, client_id, completed_event, progress_queue))
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
# Global State - Jobs Storage
# ============================================================

jobs = {}  # {job_id: {"status": ..., "queue": Queue, "result": ...}}

# ============================================================
# Background Task
# ============================================================

async def process_workflow_background(job_id: str, gray_bytes: bytes, gray_filename: str,
                                     ref_bytes: bytes, ref_filename: str, user_id: str):
    """Фоновая обработка workflow с прогрессом"""
    progress_queue = jobs[job_id]["queue"]

    async def send_progress(event_type: str, data: dict):
        await progress_queue.put({"type": event_type, **data})

    try:
        await send_progress("status", {"message": "Uploading images to ComfyUI"})

        # 1. Загрузка изображений
        gray_result = await upload_image_to_comfyui(gray_bytes, gray_filename)
        gray_name = gray_result['name']

        ref_result = await upload_image_to_comfyui(ref_bytes, ref_filename)
        ref_name = ref_result['name']

        await send_progress("status", {"message": "Images uploaded successfully"})

        # 2. Модификация workflow
        with open(WORKFLOW_PATH) as f:
            workflow = json.load(f)

        workflow["1222"]["inputs"]["image"] = gray_name
        workflow["1231"]["inputs"]["image"] = ref_name

        # 3. Постановка в очередь
        await send_progress("status", {"message": "Queueing workflow"})
        prompt_id = await queue_workflow(workflow)
        client_id = str(uuid.uuid4())

        await send_progress("queued", {"prompt_id": prompt_id, "message": "Workflow queued"})

        # 4. Ожидание завершения
        await wait_for_completion(prompt_id, client_id, progress_queue)

        # 5. Получение результата
        await send_progress("status", {"message": "Downloading result"})
        result_filename, result_bytes = await get_result_image(prompt_id)

        # 6. Сохранение локально
        save_dir = Path(STORAGE_PATH) / user_id / "comfyui" / "results"
        save_dir.mkdir(parents=True, exist_ok=True)

        save_path = save_dir / f"{prompt_id}_result.png"
        with open(save_path, 'wb') as f:
            f.write(result_bytes)

        # 7. Сохранение результата
        jobs[job_id]["result"] = {
            "prompt_id": prompt_id,
            "result_url": f"/download/{user_id}/{prompt_id}_result.png"
        }

        await send_progress("completed", {
            "prompt_id": prompt_id,
            "result_url": f"/download/{user_id}/{prompt_id}_result.png",
            "message": "Workflow completed successfully"
        })

        jobs[job_id]["status"] = "completed"

    except Exception as e:
        print(f"[COMFYUI] Error in workflow {job_id}: {e}")
        await send_progress("error", {"message": str(e)})
        jobs[job_id]["status"] = "failed"
        jobs[job_id]["error"] = str(e)

# ============================================================
# API Endpoints
# ============================================================

@app.post("/submit")
async def submit_workflow(
    gray_image: UploadFile = File(...),
    reference_image: UploadFile = File(...),
    user_id: str = Form(...)
):
    """Запуск обработки изображений, возвращает job_id для отслеживания прогресса"""
    job_id = str(uuid.uuid4())

    print(f"[COMFYUI] New job submitted: {job_id} for user {user_id}")

    # Читаем файлы
    gray_bytes = await gray_image.read()
    ref_bytes = await reference_image.read()

    # Создаем уникальные имена
    unique_id = str(uuid.uuid4())[:8]
    gray_filename = f"{unique_id}_{gray_image.filename}"
    ref_filename = f"{unique_id}_{reference_image.filename}"

    # Инициализируем job
    jobs[job_id] = {
        "status": "processing",
        "queue": asyncio.Queue(),
        "result": None,
        "error": None
    }

    print(f"[COMFYUI] Job {job_id} initialized. Total jobs: {len(jobs)}")

    # Запускаем фоновую задачу
    asyncio.create_task(process_workflow_background(
        job_id, gray_bytes, gray_filename, ref_bytes, ref_filename, user_id
    ))

    return {"job_id": job_id, "message": "Workflow submitted"}

@app.get("/progress/{job_id}")
async def get_progress(job_id: str):
    """SSE endpoint для получения прогресса задачи"""
    print(f"[COMFYUI] Progress requested for job_id: {job_id}")
    print(f"[COMFYUI] Available jobs: {list(jobs.keys())}")

    if job_id not in jobs:
        print(f"[COMFYUI] Job {job_id} not found!")
        raise HTTPException(status_code=404, detail="Job not found")

    async def event_generator():
        job = jobs[job_id]
        queue = job["queue"]

        print(f"[COMFYUI] Starting progress stream for job {job_id}")

        # Отправляем начальное событие
        yield f"data: {json.dumps({'type': 'started', 'job_id': job_id})}\n\n"

        # Стримим события из очереди
        while job["status"] == "processing":
            try:
                event = await asyncio.wait_for(queue.get(), timeout=1.0)
                yield f"data: {json.dumps(event)}\n\n"

                # Если получили событие завершения, выходим
                if event.get("type") in ["completed", "error"]:
                    break
            except asyncio.TimeoutError:
                # Отправляем heartbeat
                yield f"data: {json.dumps({'type': 'heartbeat'})}\n\n"
                continue

        # Финальное событие
        if job["status"] == "completed":
            yield f"data: {json.dumps({'type': 'done', 'result': job['result']})}\n\n"
        elif job["status"] == "failed":
            yield f"data: {json.dumps({'type': 'error', 'message': job['error']})}\n\n"

    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no"
        }
    )

@app.get("/download/{user_id}/{filename}")
async def download_result(user_id: str, filename: str):
    """Скачивание результата"""
    file_path = Path(STORAGE_PATH) / user_id / "comfyui" / "results" / filename
    return FileResponse(file_path)
