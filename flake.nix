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

        hetzner-node-watchdog = pkgs.buildGoModule rec {
          pname = "hetzner-node-watchdog";
          version = "0.1.5";

          src = ./.;
          go = pkgs.go_1_26;

          vendorHash = "sha256-6I1uig4PJnA+wOKLVGG59EMATIRk5S2F7UfsS26+MIo=";

          env.CGO_ENABLED = 0;
          ldflags = [ "-X main.version=${version}" ];

          meta = {
            description = "Restarts the Hetzner Cloud server behind a Kubernetes node that stays unavailable too long";
            homepage = "https://github.com/broeng/hetzner-node-watchdog";
            mainProgram = "hetzner-node-watchdog";
          };
        };

        # Packages deploy/ (a Helm chart) into a .tgz via `helm package`, the same
        # way `helm push`/OCI publishing or a manual install would consume it.
        # Version here is nix store metadata only; the packaged chart's actual
        # name/version come from deploy/Chart.yaml.
        chart = pkgs.stdenvNoCC.mkDerivation {
          pname = "hetzner-node-watchdog-chart";
          version = "0.1.5";

          src = ./deploy;
          nativeBuildInputs = [ pkgs.kubernetes-helm ];

          dontBuild = true;

          installPhase = ''
            runHook preInstall
            mkdir -p $out
            # hcloud.token is unset in the chart's own defaults by design (it is
            # required at install time, not baked into the chart); supply a
            # throwaway value purely so lint can render the templates.
            helm lint . --set hcloud.token=lint-placeholder
            helm package . --destination $out
            runHook postInstall
          '';

          meta = {
            description = "Helm chart for hetzner-node-watchdog";
            homepage = "https://github.com/broeng/hetzner-node-watchdog";
          };
        };
      in
      {

        packages = {
          default = hetzner-node-watchdog;
          inherit hetzner-node-watchdog chart;
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
