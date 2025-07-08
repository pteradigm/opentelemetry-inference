FROM python:3.9-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy requirements and install Python dependencies
COPY models/simple-scaler/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy model files
COPY models/ ./models/

# Expose MLServer ports
EXPOSE 8080 8081

# Set environment variables for KServe v2 protocol
ENV PROTOCOL=v2
ENV MLSERVER_HTTP_PORT=8080
ENV MLSERVER_GRPC_PORT=8081
ENV MLSERVER_MODELS_DIR=/app/models
ENV MLSERVER_HOST=0.0.0.0

# Start MLServer
CMD ["mlserver", "start", "."]