{
  description = "Flake to build by pulling from git repo using make install";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
  };

  outputs = { self, nixpkgs }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };

    # Development source: live git clone (no sha256 needed)
    devSrc = builtins.fetchGit {
      url = "https://github.com/babbage88/infra-cli.git";
      ref = "develop"; # or "main", or a feature branch
    };
  in {
    packages.${system}.default = pkgs.stdenv.mkDerivation {
      pname = "infractl";
      version = "0.0.99";

      src = devSrc;

      nativeBuildInputs = [ pkgs.makeWrapper ];
      buildInputs = [ pkgs.go pkgs.git ];

      buildPhase = ''
        make build
      '';

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
