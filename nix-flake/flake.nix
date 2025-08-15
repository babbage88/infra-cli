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

      # Fetch the Git repository
      src = pkgs.fetchFromGitHub {
        owner = "babbage88";
        repo = "infra-cli";
        rev = "eda5a97860a784d817627cb1b3a7255bf914f6e4";
        # Start with a fake but valid hash so Nix will print the real one
        sha256 = "sha256-73SniEpXdUmiq0hR1kAVzfW/MLgwHGmM2tGQ7FbokQo=";
      };

      # Dependencies for build
      nativeBuildInputs = [ pkgs.makeWrapper ];
      buildInputs = [ pkgs.go pkgs.git ];

      # We want to run the repo's Makefile
      buildPhase = ''
        make utils
      '';

      installPhase = ''
        # Nix automatically provides $out as the install destination
        make install PREFIX=$out
      '';

      meta = with pkgs.lib; {
        description = "CLI for managing hybrid infrastructure and deployments";
        license = licenses.mit;
        maintainers = []; # fill in with maintainers.<name> if desired
        platforms = platforms.linux;
      };
    };
  };
}
