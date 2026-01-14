{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { self
    , nixpkgs
    , flake-utils
    , ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "wc3ts";
          version = "0.0.1";
          src = ./.;
          vendorHash = null;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go
            go

            # Linting and Formatting
            golangci-lint
            gofumpt
            golines
            nixpkgs-fmt

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
