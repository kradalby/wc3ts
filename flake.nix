{
  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    flake-checks.url = "github:kradalby/flake-checks";
    flake-checks.inputs.nixpkgs.follows = "nixpkgs";
    flake-checks.inputs.flake-utils.follows = "flake-utils";
  };

  outputs =
    { self
    , nixpkgs
    , flake-utils
    , flake-checks
    , ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        # The Go formatters run against a `go 1.27.0` go.mod. Built with the
        # default Go (1.26.x) they try to fetch the 1.27 toolchain over the
        # network, which the Nix sandbox denies, so build them with go_latest.
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            (_final: prev: {
              gofumpt = prev.gofumpt.override {
                buildGoModule = prev.buildGoLatestModule;
              };
              gotools = prev.gotools.override {
                buildGoModule = prev.buildGoLatestModule;
                go = prev.go_latest;
              };
            })
          ];
        };
        fc = flake-checks.lib;
        common = {
          inherit pkgs;
          root = ./.;
          pname = "wc3ts";
          version = "0.0.1";
          vendorHash = "sha256-rS0Cb/TEzbpy3j1Fu3bY/XCtE2iJ4dlIl1Of4afOP/M=";
          goPkg = pkgs.go_latest;
        };
      in
      {
        packages.default = fc.goBuild common;

        formatter = fc.formatter common;

        checks = {
          build = fc.goBuild common;
          gotest = fc.goTest common;
          golangci-lint = fc.goLint common;
          formatting = fc.goFormat common;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go
            go_latest
            gopls # already built with buildGoLatestModule upstream

            # Linting and Formatting
            golangci-lint # nixpkgs already builds this with Go 1.27
            gofumpt
            gotools # provides goimports
            golines
            nixpkgs-fmt

            # Build
            gnumake

            # Pre-commit
            prek

            # Testing
            gotestsum

            # Release
            goreleaser

            # Utilities
            git
            git-absorb
          ];
        };
      }
    );
}
