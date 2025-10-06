import tensorflow as tf
print("TensorFlow version:", tf.__version__)

def create_single_model(input_dim=14, output_dim=15, timesteps=300):
    """
    Создает модель для работы с 14 входными параметрами (15-1)
    Вход: [300, 14] - временное окно без одного параметра
    Выход: [15] - текущий срез всех параметров
    """
    model = tf.keras.Sequential([
        # CNN для временных паттернов
        tf.keras.layers.Conv1D(64, kernel_size=5, activation='relu',
                               input_shape=(timesteps, input_dim)),
        tf.keras.layers.MaxPooling1D(2),
        tf.keras.layers.Conv1D(128, kernel_size=3, activation='relu'),
        tf.keras.layers.MaxPooling1D(2),

        # LSTM для временных зависимостей
        tf.keras.layers.LSTM(64, return_sequences=False),  # Только последнее состояние

        # Бутылочное горлышко
        tf.keras.layers.Dense(8, activation='relu'),

        # Выходной слой - 15 параметров
        tf.keras.layers.Dense(output_dim)  # Выход: [15]
    ])
    return model


def train_ensemble_models(models, X_train_scaled):
    """Обучает все 15 моделей ансамбля"""
    for excluded_param, model_info in models.items():
        excluded_idx = model_info['excluded_index']

        # Вход: окно без исключенного параметра [samples, 300, 14]
        X_train_partial = np.delete(X_train_scaled, excluded_idx, axis=2)

        # Цель: последний временной срез всех 15 параметров [samples, 15]
        y_train_target = X_train_scaled[:, -1, :]  # ← Берем только последний срез!

        print(f"Обучение модели без {excluded_param}")
        history = model_info['model'].fit(
            X_train_partial, y_train_target,  # ← Измененная цель
            epochs=100,
            batch_size=32,
            validation_split=0.2,
            verbose=1
        )


def detect_anomaly(ensemble_models, current_window):
    """
    current_window: текущее окно данных [300, 15]
    Возвращает: suspected_parameter, confidence_score
    """
    errors = {}

    for excluded_param, model_info in ensemble_models.items():
        excluded_idx = model_info['excluded_index']

        # Вход: окно без исключенного параметра [300, 14]
        partial_input = np.delete(current_window, excluded_idx, axis=1)

        # Предсказание: текущий срез всех 15 параметров [15]
        prediction = model_info['model'].predict(partial_input.reshape(1, 300, 14))

        # Цель: реальный текущий срез [15] (последний момент окна)
        target = current_window[-1, :]  # ← Последний временной срез

        # Считаем MSE между предсказанием и реальными данными
        mse = np.mean((prediction - target) ** 2)
        errors[excluded_param] = mse

    # Находим модель с минимальной ошибкой
    healthy_model = min(errors, key=errors.get)
    min_error = errors[healthy_model]

    # Вычисляем confidence score
    other_errors = [e for p, e in errors.items() if p != healthy_model]
    confidence = np.mean(other_errors) / min_error

    return healthy_model, confidence, errors
