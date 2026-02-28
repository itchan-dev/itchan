FROM python:3.12-alpine
RUN pip install --no-cache-dir jinja2
COPY render.py /render.py
ENTRYPOINT ["python3", "/render.py"]
