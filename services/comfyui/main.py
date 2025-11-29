import json
import uuid
import asyncio
import aiohttp
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

        print(f"[COMFYUI] Sending workflow to {COMFYUI_URL}/prompt")
        print(f"[COMFYUI] Workflow nodes: {list(workflow.keys())[:5]}...")  # Первые 5 нод

        async with session.post(f"{COMFYUI_URL}/prompt", json=payload) as resp:
            print(f"[COMFYUI] Queue response status: {resp.status}")
            result = await resp.json()
            print(f"[COMFYUI] Queue response: {result}")

            if 'prompt_id' not in result:
                raise Exception(f"No prompt_id in response: {result}")

            return result['prompt_id']

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

        # 3. Постановка workflow в очередь и ожидание через polling
        await send_progress("status", {"message": "Queueing workflow"})
        print(f"[COMFYUI] Job {job_id}: Queueing workflow to ComfyUI...")
        prompt_id = await queue_workflow(workflow)
        print(f"[COMFYUI] Job {job_id}: Workflow queued with prompt_id={prompt_id}")

        await send_progress("queued", {"prompt_id": prompt_id, "message": "Workflow queued"})

        await send_progress("status", {"message": "Waiting for completion (polling)"})
        print(f"[COMFYUI] Job {job_id}: Waiting for completion via polling...")
        await poll_history(prompt_id)

        # 6. Получение результата
        await send_progress("status", {"message": "Downloading result"})
        result_filename, result_bytes = await get_result_image(prompt_id)

        # 7. Сохранение локально
        save_dir = Path(STORAGE_PATH) / user_id / "comfyui" / "results"
        save_dir.mkdir(parents=True, exist_ok=True)

        save_path = save_dir / f"{prompt_id}_result.png"
        with open(save_path, 'wb') as f:
            f.write(result_bytes)

        # 8. Сохранение результата
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
