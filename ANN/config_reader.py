import yaml
import os


def load_tags_from_yaml(file_path='configs/tags.yaml'):
    """
    Загружает список тегов из YAML-файла конфигурации с валидацией
    """
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"Файл конфигурации не найден: {file_path}")

    with open(file_path, 'r', encoding='utf-8') as file:
        config = yaml.safe_load(file)

    if 'tags' not in config:
        raise KeyError("В конфигурационном файле отсутствует раздел 'tags'")

    tags = []
    excluded_tags = []

    for tag_name, tag_config in config['tags'].items():
        # Пропускаем закомментированные и служебные теги
        if (tag_name.startswith('#') or
                tag_name.startswith('_') or
                tag_name in ['timestamp', 'quality'] or
                not isinstance(tag_config, dict)):
            excluded_tags.append(tag_name)
            continue

        # Проверяем, что тег имеет правильный тип
        if tag_config.get('type') in ['float32', 'float64', 'int32', 'int16']:
            tags.append(tag_name)
        else:
            excluded_tags.append(tag_name)

    print(f"Загружено тегов: {len(tags)}")
    print(f"Исключено тегов: {len(excluded_tags)}")
    if excluded_tags:
        print(f"Исключенные теги: {excluded_tags}")

    return tags


# Использование
try:
    TAGS = load_tags_from_yaml('../configs/tags.yaml')
    print("Список TAGS для использования:")
    print(TAGS)
except Exception as e:
    print(f"Ошибка загрузки конфигурации: {e}")
    # Fallback на ручной список
    TAGS = ['SD1A', 'SD2', 'TC19', 'TC20', 'VGVFB', 'PT8', 'PT258',
            'ssi_TurbineTemp', 'PT9', 'TC101', 'TC102',
            'PT181A', 'PT181B', 'PT182A', 'PT182B']
