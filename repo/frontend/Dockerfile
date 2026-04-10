FROM node:20-alpine AS css-builder
WORKDIR /build
COPY tailwind.config.js ./
COPY internal/templates/*.templ ./internal/templates/
RUN npm install tailwindcss@3.4
RUN echo '@tailwind base; @tailwind components; @tailwind utilities;' > input.css
RUN npx tailwindcss -i input.css -o tailwind.min.css --minify

FROM nginx:1.25-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY frontend/static /usr/share/nginx/html/static
COPY --from=css-builder /build/tailwind.min.css /usr/share/nginx/html/static/tailwind.min.css
EXPOSE 80
