{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };

        hetzner-node-watchdog = pkgs.buildGoModule {
          pname = "hetzner-node-watchdog";
          version = "0.1.0";

          src = ./.;
          go = pkgs.go_1_26;

          vendorHash = "sha256-6I1uig4PJnA+wOKLVGG59EMATIRk5S2F7UfsS26+MIo=";

          env.CGO_ENABLED = 0;
          ldflags = [ "-X main.version=0.1.0" ];

          meta = {
            description = "Restarts the Hetzner Cloud server behind a Kubernetes node that stays unavailable too long";
            homepage = "https://github.com/broeng/hetzner-node-watchdog";
            mainProgram = "hetzner-node-watchdog";
          };
        };
      in
      {

        packages = {
          default = hetzner-node-watchdog;
          inherit hetzner-node-watchdog;
        };

        formatter = pkgs.nixfmt-tree;

        devShell = pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_26
          ];
        };
      }
    );
}
