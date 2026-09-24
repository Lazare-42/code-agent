{
  description = "code-agent — code review MCP server + Next.js frontend";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" ];
      forAll = nixpkgs.lib.genAttrs systems;
      pkgsFor = s: nixpkgs.legacyPackages.${s};
    in {
      packages = forAll (s:
        let pkgs = pkgsFor s; in {
          default = pkgs.buildGoModule {
            pname = "code-agent";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-478T/7A5V94DPG8Mldbp/Uc28wlZeWe46q61uzJnOZM=";
            subPackages = [ "cmd/code-agent" ];
            # mattn/go-sqlite3 needs cgo (enabled by default in buildGoModule).
          };

          # Next.js frontend (pnpm, standalone). Geist bundled locally so the
          # build is hermetic in the Nix sandbox.
          frontend = pkgs.stdenv.mkDerivation (finalAttrs: {
            pname = "code-agent-frontend";
            version = "0.1.0";
            src = ./frontend;
            nativeBuildInputs = [ pkgs.nodejs_22 pkgs.pnpm pkgs.pnpmConfigHook pkgs.makeWrapper ];
            pnpmDeps = pkgs.fetchPnpmDeps {
              inherit (finalAttrs) pname version src;
              fetcherVersion = 4;
              hash = "sha256-w8YdJgHcuVBgGJqxbb81gtvj0/xe8Y9kybF4DqXJAxU=";
            };
            env = {
              NEXT_TELEMETRY_DISABLED = "1";
              CI = "true";
            };
            buildPhase = ''
              runHook preBuild
              pnpm build
              runHook postBuild
            '';
            installPhase = ''
              runHook preInstall
              dir=$out/share/code-agent-frontend
              mkdir -p "$dir/.next"
              cp -r .next/standalone/. "$dir"/
              cp -r .next/static "$dir/.next/static"
              [ -d public ] && cp -r public "$dir/public" || true
              makeWrapper ${pkgs.nodejs_22}/bin/node $out/bin/code-agent-frontend \
                --add-flags "$dir/server.js"
              runHook postInstall
            '';
          });
        });
      overlays.default = final: prev: {
        code-agent = self.packages.${final.system}.default;
        code-agent-frontend = self.packages.${final.system}.frontend;
      };
    };
}
