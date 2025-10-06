from typing import Union, List
import pandas as pd
import sqlite3
from datetime import datetime

from config_reader import load_tags_from_yaml

# Параметры для сбора
# TAGS = [
#     'SD1A', 'SD2', 'TC19', 'TC20', 'VGVFB', 'PT8', 'PT258',
#     'ssi_TurbineTemp', 'PT9', 'TC101', 'TC102',
#     'PT181A', 'PT181B', 'PT182A', 'PT182B'
# ]
TAGS = load_tags_from_yaml('../configs/tags.yaml')


def fetch_plc_data(start_time: Union[str, pd.Timestamp, datetime, int],
                   end_time: Union[str, pd.Timestamp, datetime, int],
                   tags: List[str] = TAGS) -> pd.DataFrame:
    """
    Извлекает данные из БД за указанный период

    Args:
        start_time: Начальное время в одном из форматов:
                   - str: "2024-01-15 00:00:00", "2024-01-15T00:00:00"
                   - pd.Timestamp: pd.Timestamp("2024-01-15")
                   - datetime: datetime(2024, 1, 15, 0, 0, 0)
                   - int: Unix timestamp в наносекундах (прямой вход в БД)
        end_time: Конечное время (аналогичные форматы)
        tags: Список тегов для извлечения

    Returns:
        pd.DataFrame: DataFrame с данными в широком формате
    """

    # Преобразуем время в наносекунды для БД
    def to_nanoseconds(t):
        if isinstance(t, int):
            return t  # Уже в наносекундах
        elif isinstance(t, (str, pd.Timestamp, datetime)):
            return int(pd.Timestamp(t).value)
        else:
            raise ValueError(f"Неподдерживаемый тип времени: {type(t)}")

    start_ns = to_nanoseconds(start_time)
    end_ns = to_nanoseconds(end_time)

    print(f"Запрос данных: {pd.Timestamp(start_ns)} -> {pd.Timestamp(end_ns)}")

    # Далее ваш существующий код...
    conn = sqlite3.connect('../data/plc_data.db')

    query = f"""
    SELECT timestamp_ns, tag_name, value 
    FROM numeric_time_series 
    WHERE tag_name IN ({','.join('?' * len(tags))})
    AND timestamp_ns BETWEEN ? AND ?
    AND quality = 0
    ORDER BY timestamp_ns
    """

    params = tags + [start_ns, end_ns]
    df = pd.read_sql_query(query, conn, params=params)
    conn.close()

    # Преобразуем в широкий формат
    df_wide = df.pivot(index='timestamp_ns', columns='tag_name', values='value')
    df_wide.index = pd.to_datetime(df_wide.index)

    return df_wide

if __name__ == '__main__':
    print("Тестовое извлечение данных из БД")
    # 4. Unix наносекунды (прямой вход)
    df4 = fetch_plc_data(1759750819912945000, 1759754710909344500)
    print("Успешно!")



# Извлекаем данные
# df_normal = fetch_plc_data(start_normal, end_normal)
# df_incident = fetch_plc_data(start_incident, end_incident)