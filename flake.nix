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
        packages.default = (pkgs.buildGoModule.override { go = pkgs.go_1_26; }) {
          pname = "wc3ts";
          version = "0.0.1";
          src = ./.;
          vendorHash = "sha256-6L/joCTHdAr+f/5nrsptbEKZFrkJQfVEbpi/W1OBO5c=";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go
            go_1_26

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
