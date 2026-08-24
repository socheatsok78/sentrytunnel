{
  dockerTools,
  sentrytunnel,
}:
let
  version = sentrytunnel.version;
in
dockerTools.buildLayeredImage {
  name = "docker-sentrytunnel-${version}";
  tag = version;
  contents = [
    dockerTools.fakeNss
    dockerTools.caCertificates
    sentrytunnel
  ];
  config = {
    Entrypoint = [ "${sentrytunnel}/bin/sentrytunnel" ];
  };
}
