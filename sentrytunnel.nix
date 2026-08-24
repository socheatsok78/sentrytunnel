{
  buildGoModule,
}:
buildGoModule (finalAttrs: {
  pname = "sentrytunnel";
  version = "1.1.0";

  src = ./.;

  vendorHash = "sha256-FuDeGdWXtfgbidwwwtqjOnCJOGieG3IyVmYFUv3xOyM=";

  ldflags = [
    "-s"
    "-w"
    "-X github.com/socheatsok78/sentrytunnel.Version=${finalAttrs.version}"
  ];
})
