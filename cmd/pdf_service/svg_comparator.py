"""
SVG Comparison Module
Сравнивает оригинальный и изменённый SVG файлы
"""

import xml.etree.ElementTree as ET
from typing import Dict, List, Tuple
import re


# ============================================================
# SVG Element Extraction
# ============================================================

def extract_elements(svg_content: str) -> Dict[str, Dict]:
    """Извлекает элементы из SVG по их ID"""
    root = ET.fromstring(svg_content)
    elements = {}

    # Все элементы с ID
    for elem in root.iter():
        elem_id = elem.get('id')
        if elem_id:
            elements[elem_id] = {
                'tag': elem.tag.split('}')[-1],  # убираем namespace
                'attributes': dict(elem.attrib),
                'element': elem
            }

    return elements


# ============================================================
# Element Classification
# ============================================================

def classify_element(elem_id: str) -> str:
    """Определяет тип элемента по ID"""
    if elem_id.startswith('Wall_'):
        return 'стена'
    elif elem_id.startswith('Door_'):
        return 'дверь'
    elif elem_id.startswith('Window_'):
        return 'окно'
    elif elem_id.startswith('Room_'):
        return 'комната'
    elif elem_id.startswith('Balcony_'):
        return 'балкон'
    else:
        return 'элемент'


# ============================================================
# Change Detection
# ============================================================

def compare_position(attrs1: Dict, attrs2: Dict, tag: str) -> bool:
    """Сравнивает позицию элементов"""
    if tag == 'rect':
        return (attrs1.get('x') != attrs2.get('x') or
                attrs1.get('y') != attrs2.get('y'))
    elif tag == 'path':
        return attrs1.get('d') != attrs2.get('d')
    return False


def compare_color(attrs1: Dict, attrs2: Dict) -> bool:
    """Сравнивает цвет элементов"""
    style1 = attrs1.get('style', '')
    style2 = attrs2.get('style', '')
    fill1 = attrs1.get('fill', '')
    fill2 = attrs2.get('fill', '')
    stroke1 = attrs1.get('stroke', '')
    stroke2 = attrs2.get('stroke', '')

    return (style1 != style2 or fill1 != fill2 or stroke1 != stroke2)


def compare_size(attrs1: Dict, attrs2: Dict, tag: str) -> bool:
    """Сравнивает размер элементов"""
    if tag == 'rect':
        return (attrs1.get('width') != attrs2.get('width') or
                attrs1.get('height') != attrs2.get('height'))
    return False


def detect_changes(original: Dict[str, Dict], edited: Dict[str, Dict]) -> List[str]:
    """Обнаруживает изменения между двумя SVG"""
    changes = []

    # Найти добавленные элементы
    added = set(edited.keys()) - set(original.keys())
    for elem_id in added:
        elem_type = classify_element(elem_id)
        changes.append(f"Добавлен(а) {elem_type} ({elem_id})")

    # Найти удалённые элементы
    removed = set(original.keys()) - set(edited.keys())
    for elem_id in removed:
        elem_type = classify_element(elem_id)
        changes.append(f"Удалён(а) {elem_type} ({elem_id})")

    # Найти изменённые элементы
    common = set(original.keys()) & set(edited.keys())
    for elem_id in common:
        orig_elem = original[elem_id]
        edit_elem = edited[elem_id]

        elem_type = classify_element(elem_id)
        tag = orig_elem['tag']

        modifications = []

        # Проверка изменения позиции
        if compare_position(orig_elem['attributes'], edit_elem['attributes'], tag):
            modifications.append('перемещён(а)')

        # Проверка изменения цвета
        if compare_color(orig_elem['attributes'], edit_elem['attributes']):
            modifications.append('изменён(а) цвет')

        # Проверка изменения размера
        if compare_size(orig_elem['attributes'], edit_elem['attributes'], tag):
            modifications.append('изменён(а) размер')

        if modifications:
            change_desc = ', '.join(modifications)
            changes.append(f"{elem_type.capitalize()} ({elem_id}): {change_desc}")

    return changes


# ============================================================
# Public API
# ============================================================

def compare_svg_files(original_path: str, edited_path: str) -> List[str]:
    """Сравнивает два SVG файла и возвращает список изменений"""
    with open(original_path, 'r', encoding='utf-8') as f:
        original_content = f.read()

    with open(edited_path, 'r', encoding='utf-8') as f:
        edited_content = f.read()

    original_elements = extract_elements(original_content)
    edited_elements = extract_elements(edited_content)

    return detect_changes(original_elements, edited_elements)
