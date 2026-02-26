from typing import Optional, List
from sqlmodel import SQLModel, Field, Relationship
from datetime import time
from app.models.enums import RoomType, DayOfWeek

# --- Таблица связи (Many-to-Many) для Занятий и Групп ---
class CourseClassGroupLink(SQLModel, table=True):
    group_id: int | None = Field(default=None, foreign_key="group.id", primary_key=True)
    course_class_id: int | None = Field(default=None, foreign_key="courseclass.id", primary_key=True)

# --- Основные модели ---

class TimeSlot(SQLModel, table=True):
    """Слоты времени (сетка расписания). Например: ПН, 1 пара, 09:00-10:30"""
    id: int | None = Field(default=None, primary_key=True)
    day: DayOfWeek
    period_number: int  # Номер пары (1, 2, 3...)
    start_time: time
    end_time: time

    # В будущем сюда можно добавить связь с расписанием

class Room(SQLModel, table=True):
    id: int | None = Field(default=None, primary_key=True)
    name: str = Field(index=True) # Например "А-101"
    capacity: int
    type: RoomType

class Instructor(SQLModel, table=True):
    id: int | None = Field(default=None, primary_key=True)
    name: str
    
    # Связь: Один преподаватель -> Много занятий
    classes: List["CourseClass"] = Relationship(back_populates="instructor")

class Subject(SQLModel, table=True):
    id: int | None = Field(default=None, primary_key=True)
    name: str
    
    classes: List["CourseClass"] = Relationship(back_populates="subject")

class Group(SQLModel, table=True):
    id: int | None = Field(default=None, primary_key=True)
    name: str = Field(index=True)
    size: int # Важно для проверки вместимости аудитории
    
    # Связь: Группа может иметь много занятий через таблицу связи
    classes: List["CourseClass"] = Relationship(back_populates="groups", link_model=CourseClassGroupLink)

class CourseClass(SQLModel, table=True):
    """
    Сущность 'Занятие, которое нужно провести'.
    Это входные данные для Генетического Алгоритма.
    """
    id: int | None = Field(default=None, primary_key=True)
    
    # Внешние ключи
    subject_id: int = Field(foreign_key="subject.id")
    instructor_id: int = Field(foreign_key="instructor.id")
    
    # Требования к занятию (полезно для алгоритма)
    is_lecture: bool = True # Если True, можно объединять группы
    required_room_type: RoomType = RoomType.LECTURE
    duration: int = 1 # Количество слотов (обычно 1 пара)

    # Связи (ORM)
    subject: Subject = Relationship(back_populates="classes")
    instructor: Instructor = Relationship(back_populates="classes")
    groups: List[Group] = Relationship(back_populates="classes", link_model=CourseClassGroupLink)