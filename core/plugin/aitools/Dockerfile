FROM golang:1.24-alpine AS tabletools-builder

WORKDIR /src/core/plugin/aitools/tabletools
COPY core/plugin/aitools/tabletools/go.mod ./
COPY core/plugin/aitools/tabletools/cmd ./cmd
COPY core/plugin/aitools/tabletools/internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tabletools ./cmd/tabletools

FROM python:3.11-slim

WORKDIR /opt/core

ENV PATH=$PATH:/opt/core
ENV PYTHONPATH /opt/core
ENV UV_NO_CACHE=1

RUN pip install uv --no-cache-dir -i https://pypi.tuna.tsinghua.edu.cn/simple/

COPY core/plugin/aitools/pyproject.toml ./
COPY core/plugin/aitools/uv.lock ./

RUN uv sync -i https://pypi.tuna.tsinghua.edu.cn/simple/

COPY core/common ./common
COPY core/plugin/aitools ./plugin/aitools
COPY --from=tabletools-builder /out/tabletools /usr/local/bin/tabletools

RUN mkdir -p /opt/core/tabletools-data && chown -R 1000:1000 /opt/core/tabletools-data

CMD ["uv", "run", "plugin/aitools/main.py"]
