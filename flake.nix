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
        pkgs = nixpkgs.legacyPackages.${system};
        fc = flake-checks.lib;
        common = {
          inherit pkgs;
          root = ./.;
          pname = "wc3ts";
          version = "0.0.1";
          vendorHash = "sha256-NsTuAv433E8zevxrV969PqXRTLwKlF6BSVJjzAWCi94=";
          goPkg = pkgs.go_1_26;
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
            go_1_26
            gopls

            # Linting and Formatting
            golangci-lint
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
