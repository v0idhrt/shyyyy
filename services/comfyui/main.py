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

async def upload_image_to_comfyui(image_bytes: bytes, filename: str) -> str:
    """Загрузка изображения в ComfyUI"""
    async with aiohttp.ClientSession() as session:
        data = aiohttp.FormData()
        data.add_field('image', image_bytes, filename=filename)

        async with session.post(f"{COMFYUI_URL}/upload/image", data=data) as resp:
            result = await resp.json()
            return result['name']

async def queue_workflow(workflow: dict) -> str:
    """Постановка workflow в очередь ComfyUI"""
    async with aiohttp.ClientSession() as session:
        payload = {"prompt": workflow}
        async with session.post(f"{COMFYUI_URL}/prompt", json=payload) as resp:
            result = await resp.json()
            return result['prompt_id']

async def wait_for_completion(prompt_id: str, client_id: str):
    """Ожидание завершения через WebSocket"""
    uri = f"ws://localhost:8000/ws?clientId={client_id}"

    print(f"[COMFYUI] Connecting to WebSocket: {uri}")

    async with websockets.connect(uri, open_timeout=300, close_timeout=300) as ws:
        print(f"[COMFYUI] WebSocket connected, waiting for execution...")

        while True:
            try:
                message = await asyncio.wait_for(ws.recv(), timeout=300)
                data = json.loads(message)

                msg_type = data.get('type')

                if msg_type == 'executing':
                    node = data.get('data', {}).get('node')
                    print(f"[COMFYUI] Executing node: {node}")

                if msg_type == 'progress':
                    value = data.get('data', {}).get('value', 0)
                    max_val = data.get('data', {}).get('max', 100)
                    print(f"[COMFYUI] Progress: {value}/{max_val}")

                if msg_type == 'executed':
                    if data.get('data', {}).get('prompt_id') == prompt_id:
                        print(f"[COMFYUI] Workflow completed!")
                        return

            except asyncio.TimeoutError:
                raise TimeoutError("WebSocket timeout: workflow took longer than 300 seconds")

async def get_result_image(prompt_id: str) -> tuple:
    """Получение результата из history"""
    async with aiohttp.ClientSession() as session:
        async with session.get(f"{COMFYUI_URL}/history/{prompt_id}") as resp:
            history = await resp.json()

            # Парсим outputs ноды 1239 (SaveImage)
            outputs = history[prompt_id]['outputs']['1239']
            filename = outputs['images'][0]['filename']

            # Скачиваем изображение
            async with session.get(f"{COMFYUI_URL}/view?filename={filename}&type=output") as img_resp:
                return filename, await img_resp.read()

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

    print(f"[COMFYUI] Uploading gray image: {gray_image.filename}")
    gray_name = await upload_image_to_comfyui(gray_bytes, gray_image.filename)
    print(f"[COMFYUI] Gray image uploaded as: {gray_name}")

    print(f"[COMFYUI] Uploading reference image: {reference_image.filename}")
    ref_name = await upload_image_to_comfyui(ref_bytes, reference_image.filename)
    print(f"[COMFYUI] Reference image uploaded as: {ref_name}")

    # 2. Модификация workflow
    with open(WORKFLOW_PATH) as f:
        workflow = json.load(f)

    workflow["1222"]["inputs"]["image"] = gray_name
    workflow["1231"]["inputs"]["image"] = ref_name

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
