variable "GITHUB_REPOSITORY" {
    default = "socheatsok78/sentry-tunnel"
}

variable "GO_VERSION" { default = "1.23" }
variable "ALPINE_VERSION" { default = "3.19" }

target "docker-metadata-action" {}
target "github-metadata-action" {}

target "default" {
    inherits = [
        "docker-metadata-action",
        "github-metadata-action",
    ]
    args = {
        GO_VERSION = "${GO_VERSION}"
        ALPINE_VERSION = "${ALPINE_VERSION}"
    }
    platforms = [
        "linux/amd64",
        "linux/arm64"
    ]
}

target "dev" {
    args = {
        GO_VERSION = "${GO_VERSION}"
        ALPINE_VERSION = "${ALPINE_VERSION}"
    }
    tags = [
        "${GITHUB_REPOSITORY}:dev"
    ]
}
