"""
PDF Service
Генерация PDF отчётов по изменениям планировок
"""

import os
import base64
from datetime import datetime
from pathlib import Path
from io import BytesIO

from flask import Flask, request, send_file, jsonify
from weasyprint import HTML
import cairosvg
import requests

from svg_comparator import compare_svg_files


# ============================================================
# Configuration
# ============================================================

app = Flask(__name__)

PORT = int(os.getenv('PORT', 3004))
AUTH_SERVICE_URL = os.getenv('AUTH_SERVICE_URL', 'http://localhost:3002')
SOURCE_DIR = Path(__file__).parent.parent.parent / 'source'


# ============================================================
# SVG Processing
# ============================================================

def process_svg_to_blueprint(svg_content: bytes) -> bytes:
    """Обрабатывает SVG для создания чистого черно-белого плана"""
    import xml.etree.ElementTree as ET

    # Парсинг SVG
    root = ET.fromstring(svg_content)

    # Namespace для SVG
    ns = {'svg': 'http://www.w3.org/2000/svg'}

    # Убрать все defs/style секции
    for defs in root.findall('.//svg:defs', ns):
        root.remove(defs)

    for style in root.findall('.//svg:style', ns):
        root.remove(style)

    # Обработка всех элементов
    for elem in root.iter():
        tag = elem.tag.split('}')[-1] if '}' in elem.tag else elem.tag

        if tag in ['path', 'rect', 'polygon', 'polyline', 'line', 'circle', 'ellipse']:
            # Удалить class атрибуты
            if 'class' in elem.attrib:
                del elem.attrib['class']

            # Определить тип элемента по ID
            elem_id = elem.get('id', '')

            # Стены - черный контур
            if 'Wall' in elem_id or 'wall' in elem_id.lower():
                elem.set('fill', 'none')
                elem.set('stroke', '#000000')
                elem.set('stroke-width', '3')

            # Двери - тонкая линия
            elif 'Door' in elem_id or 'door' in elem_id.lower():
                elem.set('fill', 'none')
                elem.set('stroke', '#000000')
                elem.set('stroke-width', '2')
                elem.set('stroke-dasharray', '5,5')

            # Окна - двойная линия
            elif 'Window' in elem_id or 'window' in elem_id.lower():
                elem.set('fill', 'none')
                elem.set('stroke', '#000000')
                elem.set('stroke-width', '2')

            # Балконы - пунктирная линия
            elif 'Balcony' in elem_id or 'balcony' in elem_id.lower():
                elem.set('fill', 'none')
                elem.set('stroke', '#666666')
                elem.set('stroke-width', '2')
                elem.set('stroke-dasharray', '4,4')

            # Комнаты - светло-серая заливка
            elif 'Room' in elem_id or 'room' in elem_id.lower():
                elem.set('fill', '#f0f0f0')
                elem.set('stroke', '#cccccc')
                elem.set('stroke-width', '1')

            # Остальное - базовый стиль
            else:
                if elem.get('fill') and elem.get('fill') != 'none':
                    elem.set('fill', '#e8e8e8')
                elem.set('stroke', '#333333')
                elem.set('stroke-width', '1')

    return ET.tostring(root, encoding='utf-8')


def svg_to_png_base64(svg_path: str, output_width: int = 800) -> str:
    """Конвертирует SVG в PNG с улучшенным качеством"""
    import xml.etree.ElementTree as ET

    with open(svg_path, 'rb') as svg_file:
        svg_data = svg_file.read()

    # Обработка SVG в черно-белый план
    processed_svg = process_svg_to_blueprint(svg_data)

    # Получить размеры SVG для правильного масштабирования
    root = ET.fromstring(processed_svg)
    svg_width = float(root.get('width', 2479))
    svg_height = float(root.get('height', 3508))

    # Вычислить пропорциональную высоту
    output_height = int(output_width * (svg_height / svg_width))

    # Конвертация SVG → PNG с фиксированными размерами
    png_data = cairosvg.svg2png(
        bytestring=processed_svg,
        output_width=output_width,
        output_height=output_height,
        background_color='white'
    )

    # Кодирование в base64
    return base64.b64encode(png_data).decode('utf-8')


# ============================================================
# User Data Fetching
# ============================================================

def fetch_user_data(user_id: str) -> dict:
    """Получает данные пользователя из Auth Service"""
    try:
        response = requests.get(f'{AUTH_SERVICE_URL}/internal/users/{user_id}', timeout=5)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"Error fetching user data: {e}")
        # Возвращаем стандартные данные если не удалось получить
        return {
            'fio': 'Не указано',
            'phone': 'Не указано',
            'address': 'Не указано'
        }


# ============================================================
# PDF Generation
# ============================================================

def generate_pdf_report(file_id: str, user_id: str) -> BytesIO:
    """Генерирует PDF отчёт о изменениях планировки"""

    # Пути к SVG файлам
    original_svg = SOURCE_DIR / f'{user_id}/svg/{file_id}.svg'
    edited_svg = SOURCE_DIR / f'{user_id}/svg/edited/{file_id}.svg'

    if not original_svg.exists():
        raise FileNotFoundError(f'Оригинальный файл не найден: {original_svg}')

    if not edited_svg.exists():
        raise FileNotFoundError(f'Изменённый файл не найден: {edited_svg}')

    # Конвертация SVG → PNG (base64)
    original_png_b64 = svg_to_png_base64(str(original_svg))
    edited_png_b64 = svg_to_png_base64(str(edited_svg))

    # Сравнение SVG и получение списка изменений
    changes = compare_svg_files(str(original_svg), str(edited_svg))

    # Получение данных пользователя
    user_data = fetch_user_data(user_id)

    # Подготовка данных для шаблона
    template_data = {
        'date': datetime.now().strftime('%d.%m.%Y'),
        'original_png': original_png_b64,
        'edited_png': edited_png_b64,
        'changes': changes,
        'fio': user_data.get('fio', 'Не указано'),
        'phone': user_data.get('phone', 'Не указано'),
        'address': user_data.get('address', 'Не указано'),
    }

    # Чтение HTML шаблона
    template_path = Path(__file__).parent / 'templates/report.html'
    with open(template_path, 'r', encoding='utf-8') as f:
        template = f.read()

    # Простая подстановка переменных (используем Jinja2-like синтаксис)
    html_content = template
    for key, value in template_data.items():
        if isinstance(value, list):
            # Обработка списка изменений
            changes_html = '\n'.join([f'<li>{change}</li>' for change in value])
            html_content = html_content.replace('{% if changes %}', '')
            html_content = html_content.replace('{% else %}', '<!--')
            html_content = html_content.replace('{% endif %}', '-->')
            html_content = html_content.replace('{% for change in changes %}', '')
            html_content = html_content.replace('{% endfor %}', '')
            html_content = html_content.replace('{{ change }}', changes_html)
        else:
            html_content = html_content.replace('{{ ' + key + ' }}', str(value))

    # Генерация PDF через WeasyPrint
    pdf_buffer = BytesIO()
    HTML(string=html_content).write_pdf(pdf_buffer)
    pdf_buffer.seek(0)

    # Сохранение PDF в папку пользователя
    pdf_dir = SOURCE_DIR / f'{user_id}/pdf'
    pdf_dir.mkdir(parents=True, exist_ok=True)

    pdf_filename = f'report_{file_id}_{datetime.now().strftime("%Y%m%d_%H%M%S")}.pdf'
    pdf_path = pdf_dir / pdf_filename

    with open(pdf_path, 'wb') as f:
        f.write(pdf_buffer.getvalue())

    pdf_buffer.seek(0)
    return pdf_buffer


# ============================================================
# HTTP Handlers
# ============================================================

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({'status': 'ok', 'service': 'pdf-service'}), 200


@app.route('/generate', methods=['POST'])
def generate():
    """
    Генерация PDF отчёта
    Body: { "file_id": "1", "user_id": "demo-user" }
    """
    data = request.get_json()

    if not data:
        return jsonify({'error': 'Request body is required'}), 400

    file_id = data.get('file_id')
    user_id = data.get('user_id', 'demo-user')

    if not file_id:
        return jsonify({'error': 'file_id is required'}), 400

    try:
        pdf_buffer = generate_pdf_report(file_id, user_id)
        return send_file(
            pdf_buffer,
            mimetype='application/pdf',
            as_attachment=True,
            download_name=f'report_{file_id}_{datetime.now().strftime("%Y%m%d_%H%M%S")}.pdf'
        )
    except FileNotFoundError as e:
        return jsonify({'error': str(e)}), 404
    except Exception as e:
        print(f"Error generating PDF: {e}")
        return jsonify({'error': f'Internal server error: {str(e)}'}), 500


# ============================================================
# Main
# ============================================================

if __name__ == '__main__':
    print(f"PDF Service starting on port {PORT}")
    print(f"Auth Service URL: {AUTH_SERVICE_URL}")
    print(f"Source directory: {SOURCE_DIR}")
    app.run(host='0.0.0.0', port=PORT, debug=True)
