from enum import Enum

class RoomType(str, Enum):
    LECTURE = "lecture"   # Лекционная
    PRACTICE = "practice" # Практическая/Семинарская
    LAB = "lab"           # Лабораторная

class DayOfWeek(str, Enum):
    MONDAY = "monday"
    TUESDAY = "tuesday"
    WEDNESDAY = "wednesday"
    THURSDAY = "thursday"
    FRIDAY = "friday"
    SATURDAY = "saturday"