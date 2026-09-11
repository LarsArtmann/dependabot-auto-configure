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

    go-error-family = {
      url = "github:LarsArtmann/go-error-family/v0.10.0";
      flake = false;
    };

    go-finding = {
      # v1.10.0 is the first tag carrying the toolsdk/ sub-module.
      url = "github:LarsArtmann/go-finding?ref=refs/tags/v1.10.0";
      flake = false;
    };

    linter-autoconfigure-sdk = {
      url = "github:LarsArtmann/linter-autoconfigure-sdk?ref=refs/tags/v0.1.0";
      flake = false;
    };
  };

  outputs =
    inputs@{
      self,
      flake-parts,
      go-atomic-write,
      go-error-family,
      go-finding,
      linter-autoconfigure-sdk,
      ...
    }:
    let
      version = self.rev or self.dirtyRev or "dev";
    in
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      go-standard = {
        pname = "dependabot-auto-configure";
        vendorHash = "sha256-xKDWFMjlUmCosM3U6GpGJGd0uO1UCS5ADbVtAR0cFak=";

        description = "Auto-configure .github/dependabot.yml for the detected repository shape";
        enableCheck = false;
        subPackages = [ "cmd/dependabot-auto-configure" ];

        deps = {
          "github.com/larsartmann/go-atomic-write" = go-atomic-write;
          "github.com/larsartmann/go-error-family" = go-error-family;
          "github.com/larsartmann/go-finding" = go-finding;
          "github.com/larsartmann/linter-autoconfigure-sdk" = linter-autoconfigure-sdk;
        };

        src = inputs.nixpkgs.lib.fileset.toSource {
          root = ./.;
          fileset = inputs.nixpkgs.lib.fileset.unions [
            ./go.mod
            ./go.sum
            ./cmd
            ./internal
            ./pkg
          ];
        };

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

        enableNixfmt = true;
      };
    };
}
