{ pkgs ? import <nixpkgs> {} }:

pkgs.buildGoModule {
  pname = "discord-ptt-go";
  version = "0.1.0";
  src = ./.;
  subPackages = [ "." ];
  vendorHash = "sha256-4HDlQjrCAgiblScGfYdeDsCGCH40jhwSggQZ3GlCyX8=";

  buildInputs = [
    pkgs.xdotool
  ];
}
