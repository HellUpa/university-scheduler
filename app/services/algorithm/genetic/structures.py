from dataclasses import dataclass
from typing import List
import random

@dataclass
class Gene:
    """
    Ген представляет собой одно конкретное назначение:
    Какое занятие (class_id) проходит в какой аудитории (room_id) и когда (slot_id).
    """
    class_id: int
    room_id: int
    slot_id: int
    
    # Для оптимизации кэшируем связанные данные, чтобы не искать их каждый раз
    # В реальном коде сюда можно добавить ссылки на объекты, но пока хватит ID
    instructor_id: int = 0
    group_ids: List[int] = None
    subject_id: int = 0
    students_count: int = 0 

@dataclass
class Individual:
    """
    Индивид (Хромосома) — это вариант полного расписания.
    """
    genes: List[Gene]
    fitness: float = 0.0
    
    def __post_init__(self):
        if self.group_ids is None:
            self.group_ids = []

    # Сравнение для сортировки популяций
    def __lt__(self, other):
        return self.fitness > other.fitness  # Чем больше fitness, тем лучше (если максимизируем)
        # Или self.fitness < other.fitness, если минимизируем штрафы (penalty).
        # Договоримся: Fitness = 1 / (1 + penalty). Чем выше, тем лучше.