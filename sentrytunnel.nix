{
  buildGoModule,
}:
buildGoModule (finalAttrs: {
  pname = "sentrytunnel";
  version = "1.0.0";

  src = ./.;

  vendorHash = "sha256-3GGRuXhFqlvzWaJ9axAYXjv1l2B1GEQTpIW9Kg/09tQ=";

  ldflags = [
    "-s"
    "-w"
    "-X github.com/socheatsok78/sentrytunnel.Version=${finalAttrs.version}"
  ];
})
