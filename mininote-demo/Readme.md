# MiniNote Application

A simple note-taking application built with Spring Boot and MongoDB, demonstrating containerization with Docker.

## Tech Stack

- **Backend**: Spring Boot 2.3.7 (Java 11)
- **Database**: MongoDB
- **Template Engine**: FreeMarker
- **Build Tool**: Maven

## Prerequisites

- Java 11 or higher
- Maven 3.6+
- Docker & Docker Compose (for containerized deployment)
- MongoDB (for local development without Docker)

---

## Local Development

### Option 1: Run with Local MongoDB

**1. Start MongoDB locally**

Choose one of the following methods:

```bash
# macOS (using Homebrew)
brew services start mongodb-community

# Linux (systemd)
sudo systemctl start mongod

# Or run MongoDB in Docker
docker run -d --name mongo-local -p 27017:27017 mongo:6.0
```

**2. Build the application**

```bash
cd mininote-demo
mvn clean package -DskipTests
```

**3. Run the application**

```bash
mvn spring-boot:run

# Or run the JAR directly
java -jar target/kubernetes-cafe.jar
```

**4. Access the application**

Open http://localhost:8080 in your browser.

### Option 2: Configure Custom MongoDB Connection

Edit `src/main/resources/application.yml`:

```yaml
spring:
  data:
    mongodb:
      uri: mongodb://your-mongo-host:27017/your-database
```

Or set the environment variable:

```bash
export MONGO_URL=mongodb://your-mongo-host:27017/your-database
mvn spring-boot:run
```

---

## Docker Deployment

### Manual Docker Setup

If you prefer to run containers manually:

#### Step 1: Build the Docker Image

The Dockerfile uses multi-stage build - Maven runs inside Docker, so you don't need it locally.

```bash
cd mininote-demo
docker build -t mininote-java:1.0.0 .
```

> **Note:** First build may take a few minutes to download dependencies.

#### Step 3: Create a Docker Network

```bash
docker network create mininote-network
```

#### Step 4: Run MongoDB Container

```bash
docker run -d \
  --name mininote-mongo \
  --network mininote-network \
  -p 27017:27017 \
  -v mongo-data:/data/db \
  mongo:6.0
```

**MongoDB Options Explained:**
- `-d`: Run in detached mode (background)
- `--name`: Container name for easy reference
- `--network`: Connect to the custom network
- `-p 27017:27017`: Expose MongoDB port to host
- `-v mongo-data:/data/db`: Persist data in a Docker volume

#### Step 5: Run the Spring Boot Application

```bash
docker run -d \
  --name mininote-app \
  --network mininote-network \
  -p 8080:8080 \
  -e MONGO_URL=mongodb://mininote-mongo:27017/mininote \
  mininote-java:1.0.0
```

**Application Options Explained:**
- `-e MONGO_URL=...`: Set MongoDB connection string
- The hostname `mininote-mongo` resolves within the Docker network

#### Step 6: Verify the Containers

```bash
# Check running containers
docker ps

# Check application logs
docker logs -f mininote-app

# Check MongoDB logs
docker logs -f mininote-mongo
```

#### Step 7: Access the Application

Open http://localhost:8080

#### Step 8: Stop and Clean Up

```bash
# Stop containers
docker stop mininote-app mininote-mongo

# Remove containers
docker rm mininote-app mininote-mongo

# Remove network
docker network rm mininote-network

# Remove data volume (optional - this deletes all data!)
docker volume rm mongo-data
```

### Using Docker Compose

The easiest way to run the complete application stack. The Dockerfile uses multi-stage build, so **Maven is not required locally**.

#### Step 1: Build and start all services

```bash
cd mininote-demo

# Start MongoDB and build/run the application (Maven runs inside Docker)
docker-compose up --build
```

#### Step 2: Access the application

Open http://localhost:8080

#### Step 3: Stop all services

```bash
docker-compose down

# To also remove the MongoDB data volume
docker-compose down -v
```
---

## Pushing to Docker Registry

### Docker Hub

```bash
# Tag the image
docker tag mininote-java:1.0.0 <your-dockerhub-username>/mininote-java:1.0.0

# Login to Docker Hub
docker login

# Push the image
docker push <your-dockerhub-username>/mininote-java:1.0.0
```

---

## Application Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Main page - view all notes |
| `/note` | POST | Create a new note |
| `/delete` | POST | Delete a note |
| `/actuator/health` | GET | Health check endpoint |

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MONGO_URL` | `mongodb://localhost:27017/dev` | MongoDB connection URI |
| `JAVA_OPTS` | - | JVM options (e.g., `-Xmx512m`) |

### Application Properties

Located in `src/main/resources/application.yml`:

```yaml
spring:
  data:
    mongodb:
      uri: ${MONGO_URL:mongodb://localhost:27017/dev}
  servlet:
    multipart:
      max-file-size: 1048576KB
      max-request-size: 1048576KB
```
---
