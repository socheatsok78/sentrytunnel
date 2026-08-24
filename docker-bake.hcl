variable "GITHUB_REPOSITORY" {
    default = "socheatsok78/sentrytunnel"
}

variable "ALPINE_VERSION" { default = "" }

target "docker-metadata-action" {}
target "github-metadata-action" {}

target "sentrytunnel" {
    dockerfile = "flake.nix"
    target = "sentrytunnel-image"
}

target "default" {
    inherits = [
        "docker-metadata-action",
        "github-metadata-action",
        "sentrytunnel",
    ]
    platforms = [
        "linux/amd64",
        "linux/arm64"
    ]
}

target "dev" {
    inherits = [
        "sentrytunnel",
    ]
    tags = [
        "${GITHUB_REPOSITORY}:dev"
    ]
}
