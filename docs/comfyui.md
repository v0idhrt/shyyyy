# ComfyUI Service

- POST `/submit` — загрузка серого и референс-изображений, создаёт `job_id`, кладёт задачу в очередь и запускает ComfyUI workflow.
- GET `/progress/{job_id}` — SSE, читает из очереди прогресса; теперь прогресс формируется только через polling `/history`, WebSocket не используется.
- GET `/download/{user_id}/{filename}` — отдаёт сохранённый результат из `source/<user_id>/comfyui/results`.
- Workflow: подставляет загруженные имена в ноды `1222` (gray) и `1231` (reference), ждёт завершения ноды `1239` по `/history/{prompt_id}`, затем скачивает файл через `/view`.
- Конфиг: `COMFYUI_URL=http://localhost:8000`, `STORAGE_PATH=/home/v0idhrt/Документы/govno/shyyyy/source`, `WORKFLOW_PATH=workflow.json`.
