# ====================
# Stage 1: Build
# ====================
FROM maven:3.8-eclipse-temurin-11 AS builder

WORKDIR /build

# Copy pom.xml first to cache dependencies
COPY pom.xml .
RUN mvn dependency:go-offline -B

# Copy source code and build
COPY src ./src
RUN mvn clean package -DskipTests -B

# ====================
# Stage 2: Runtime
# ====================
FROM eclipse-temurin:11-jre

WORKDIR /opt

# Install curl for healthcheck and create non-root user
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r demo && useradd -r -s /bin/false -g demo demo

# Copy the jar file from builder stage
COPY --from=builder /build/target/*.jar /opt/app.jar

# Change ownership to non-root user
RUN chown -R demo:demo /opt

# Switch to non-root user
USER demo

# Expose the application port
EXPOSE 8080

# Healthcheck to verify the application is running
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/actuator/health || exit 1

# Execute java inside the container
ENTRYPOINT ["java", "-jar", "/opt/app.jar"]