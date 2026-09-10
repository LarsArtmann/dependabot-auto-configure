{
  description = "Auto-configure .github/dependabot.yml for the detected repository shape";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    go-nix-helpers = {
      url = "github:LarsArtmann/go-nix-helpers/a97742e806193cd7e4c457439c7e117a6cfd1fe7";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    go-atomic-write = {
      url = "github:LarsArtmann/go-atomic-write/v0.5.1";
      flake = false;
    };

    go-finding = {
      url = "github:LarsArtmann/go-finding/v1.9.2";
      flake = false;
    };
  };

  outputs =
    inputs@{
      self,
      flake-parts,
      ...
    }:
    let
      version = self.rev or self.dirtyRev or "dev";
      commit = self.shortRev or self.dirtyShortRev or "unknown";
      date = builtins.substring 0 8 (self.lastModifiedDate or "19700101");
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];

      perSystem.go-standard = {
        moduleName = "github.com/larsartmann/dependabot-auto-configure";

        enableNixfmt = true;
      };

      perSystem =
        {
          config,
          pkgs,
          lib,
          ...
        }:
        {
          packages.default = config.packages."dependabot-auto-configure";

          packages."dependabot-auto-configure" = config.goBuild {
            src = ./.;
            moduleName = "github.com/larsartmann/dependabot-auto-configure";
            mainPackage = "./cmd/dependabot-auto-configure";

            ldflags = [
              "-s"
              "-w"
              "-X github.com/larsartmann/dependabot-auto-configure/internal/cli.Version=${version}"
            ];

            extraBuildAttrs.preBuild = "export GOEXPERIMENT=jsonv2";

            shellExtraEnv = {
              GOEXPERIMENT = "jsonv2";
            };

            devShellExtraPackages = pkgs: [
              pkgs.gopls
              pkgs.gotools
              pkgs.golangci-lint
            ];
          };

          checks.test = config.packages.default.overrideAttrs (_old: {
            doCheck = true;
          });
        };
    };
}
