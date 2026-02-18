FROM python:3.11-slim

WORKDIR /app

COPY problem-app.py .

# Make the script executable
RUN chmod +x problem-app.py

# Expose the port
EXPOSE 8080

# Run the app
CMD ["python3", "problem-app.py"]
