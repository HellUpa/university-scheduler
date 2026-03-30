FROM node:lts-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

EXPOSE 5173

# Запуск Vite с флагом --host, чтобы он был доступен извне контейнера
CMD ["npm", "run", "dev", "--", "--host"]