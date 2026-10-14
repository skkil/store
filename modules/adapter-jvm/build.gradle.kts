plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.protobuf)
    alias(libs.plugins.spotless)
    `java-library`
    `maven-publish`
}

group = "org.skkil.store"
version = "0.1.0"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(25)
    }
    withSourcesJar()
}

repositories {
    mavenCentral()
}

dependencies {
    implementation(platform(libs.spring.boot.bom))
    api(libs.protobuf.java)
    api("org.springframework:spring-jdbc")
    api("org.springframework:spring-tx")
    api("tools.jackson.core:jackson-databind")
    implementation(libs.protobuf.java.util)
    implementation(libs.nimbus.jose.jwt)
    implementation("org.slf4j:slf4j-api")
    compileOnly("org.springframework.boot:spring-boot-autoconfigure")

    testImplementation(platform(libs.spring.boot.bom))
    testImplementation("org.springframework.boot:spring-boot-autoconfigure")
    testImplementation("org.springframework.boot:spring-boot-test")
    testImplementation("org.assertj:assertj-core")
    testImplementation("org.jetbrains.kotlin:kotlin-reflect")
    testImplementation("org.jetbrains.kotlin:kotlin-test-junit5")
    testImplementation("org.junit.jupiter:junit-jupiter-params")
    testImplementation("org.testcontainers:testcontainers-junit-jupiter")
    testImplementation("org.testcontainers:testcontainers-postgresql")
    testImplementation("org.postgresql:postgresql")
    testRuntimeOnly("org.slf4j:slf4j-simple")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

kotlin {
    explicitApi()
    compilerOptions {
        freeCompilerArgs.addAll("-Xjsr305=strict")
    }
}

sourceSets {
    main {
        proto {
            srcDir("../protocol/proto")
        }
    }
}

protobuf {
    protoc {
        artifact = libs.protoc.get().toString()
    }
}

spotless {
    kotlin {
        target("src/**/*.kt")
        ktlint(libs.versions.ktlint.get())
    }
    kotlinGradle {
        ktlint(libs.versions.ktlint.get())
    }
}

tasks.withType<Test> {
    useJUnitPlatform()
    systemProperty("store.migrations", layout.projectDirectory.dir("../sql/migrations").asFile.absolutePath)
}

tasks.register<JavaExec>("syncTokenKeys") {
    description = "Prints a new ES256 key pair for signing sync tokens in development."
    group = "application"
    classpath = sourceSets.main.get().runtimeClasspath
    mainClass = "org.skkil.store.token.SyncTokenKeyGeneratorKt"
}

publishing {
    publications {
        create<MavenPublication>("maven") {
            artifactId = "store-adapter-jvm"
            from(components["java"])
            pom {
                name = "store-adapter-jvm"
                description = "Write path, Push handler and sync tokens for skkil store on Spring servers."
            }
        }
    }
}
