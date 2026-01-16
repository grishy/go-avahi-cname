{
  description = "Publish CNAME records for the local host over mDNS using Avahi";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "2.4.0";
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "go-avahi-cname";
            inherit version;

            src = ./.;

            vendorHash = "sha256-Q7/EH/o1q7HQo81nMI7lIKgJ0OOo257OvuAIphSUZVI=";

            ldflags = [
              "-s" "-w"
              "-X main.version=${version}"
              "-X main.commit=${self.shortRev or "dirty"}"
              "-X main.date=1970-01-01T00:00:00Z"
            ];

            meta = with pkgs.lib; {
              description = "Publish CNAME records pointing to local host over mDNS via Avahi";
              homepage = "https://github.com/grishy/go-avahi-cname";
              license = licenses.mit;
              maintainers = [ ];
              platforms = platforms.unix; # Builds on macOS, runs on Linux (requires Avahi)
              mainProgram = "go-avahi-cname";
            };
          };

          go-avahi-cname = self.packages.${system}.default;
        };

        # TODO(human): Implement devShell with development tools
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Add development tools here
          ];
        };
      }
    );
}
