{
  description = "OpenWebUI Content Sync - A tool to sync content from various sources to OpenWebUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          openwebui-content-sync = pkgs.buildGoModule {
            pname = "openwebui-content-sync";
            version = "0.1.0";

            src = ./.;

            vendorHash = "sha256-ufD+D9Eq20JN0/iLBbbALPmYixL0dOVV/JGqEkwdp5U=";

            ldflags = [
              "-s"
              "-w"
            ];

            meta = with pkgs.lib; {
              description = "Sync content from GitHub, Confluence, Jira, Slack, and local folders to OpenWebUI";
              homepage = "https://github.com/castai/openwebui-content-sync";
              license = licenses.asl20;
              maintainers = [ ];
              mainProgram = "openwebui-content-sync";
            };
          };

          default = self.packages.${system}.openwebui-content-sync;
        };

        apps = {
          openwebui-content-sync = {
            type = "app";
            program = "${self.packages.${system}.openwebui-content-sync}/bin/openwebui-content-sync";
          };
          default = self.apps.${system}.openwebui-content-sync;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
          ];
        };
      }
    );
}
