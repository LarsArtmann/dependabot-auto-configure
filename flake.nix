{
  description = "Auto-configure .github/dependabot.yml for the detected repository shape";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    go-nix-helpers = {
      url = "github:LarsArtmann/go-nix-helpers/19fc8e5986d59f57ff8780fcbc5623fb412c345f";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    go-atomic-write = {
      url = "github:LarsArtmann/go-atomic-write/v0.5.2";
      flake = false;
    };

    go-error-family = {
      url = "github:LarsArtmann/go-error-family/v0.10.1";
      flake = false;
    };

    go-finding = {
      # v1.10.0 is the first tag carrying the toolsdk/ sub-module.
      url = "github:LarsArtmann/go-finding?ref=refs/tags/v1.11.0";
      flake = false;
    };

    linter-autoconfigure-sdk = {
      url = "github:LarsArtmann/linter-autoconfigure-sdk?ref=refs/tags/v0.2.0";
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
        goPkgAttr = "go_1_27";
        vendorHash = "sha256-V0nvJDKLRdwj2A31ZlNDZxYQ04bLCKde3IG6c5dpePQ=";

        # Nixpkgs 26.11 dropped x86_64-darwin, so `nix flake check
        # --all-systems` fails while evaluating it; ship only supported systems.
        systems = [
          "x86_64-linux"
          "aarch64-linux"
          "aarch64-darwin"
        ];

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
          pkgs.dprint
          pkgs.gopls
          pkgs.gotools
          pkgs.golangci-lint
        ];

        enableNixfmt = true;
        # goimports wraps the nixpkgs Go toolchain and tries to download a
        # newer one for the go.mod floor (impossible in the sandbox); gofumpt
        # is a standalone binary and already covers Go import formatting.
        enableGoimports = false;
      };
    };
}
