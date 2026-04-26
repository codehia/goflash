{ pkgs, ... }: {
  packages = [ pkgs.git ];
  languages = {
    go.enable = true;
    nix.enable = true;
  };
}
