from collections.abc import AsyncGenerator
from sqlalchemy.ext.asyncio import create_async_engine
from sqlalchemy.orm import sessionmaker
from sqlmodel.ext.asyncio.session import AsyncSession
from app.core.config import settings

# Создаем асинхронный движок
# echo=True полезно при разработке, чтобы видеть SQL запросы в консоли
engine = create_async_engine(
    str(settings.SQLALCHEMY_DATABASE_URI), 
    echo=True, 
    future=True
)

# Создаем фабрику сессий
async_session_maker = sessionmaker(
    engine, 
    class_=AsyncSession, 
    expire_on_commit=False
)

# Функция для получения сессии (Dependency Injection для FastAPI)
async def get_session() -> AsyncGenerator[AsyncSession, None]:
    async with async_session_maker() as session:
        yield session