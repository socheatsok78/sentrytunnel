{
  pkgs ? import <nixpkgs> { },
}:
rec {
  default = sentrytunnel;

  sentrytunnel = pkgs.callPackage ./sentrytunnel.nix { };
  sentrytunnel-image = pkgs.callPackage ./sentrytunnel-image.nix {
    inherit sentrytunnel;
  };
}
