{
  description = "Flake to build by pulling from git repo using make install";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05"; # or unstable
  };

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
  in {
    packages.${system}.default = pkgs.stdenv.mkDerivation {
      pname = "infractl";
      version = "0.0.99";

      # Fetch the Git repository (fake hash for now)
      src = pkgs.fetchFromGitHub {
        owner = "babbage88";
        repo = "infra-cli";
        rev = "develop"; # or a commit hash for production
        sha256 = "sha256-LKpIEIvQtc2vnfOKCQbS//hwVY7GCvTrb8M6X5Fi2wA="; # placeholder — Nix will tell you the real one
      };

      # Build dependencies
      nativeBuildInputs = [ pkgs.makeWrapper ];
      buildInputs = [ pkgs.go pkgs.git pkgs.makeWrapper pkgs.bash];

      # Build phase
      buildPhase = ''
        make build
      '';

      # Install phase
      installPhase = ''
        make install PREFIX=$out
      '';

      meta = with pkgs.lib; {
        description = "CLI for managing hybrid infrastructure and deployments";
        license = licenses.mit;
        maintainers = [];
        platforms = platforms.linux;
      };
    };
  };
}
