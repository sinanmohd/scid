{
  lib,
  buildGoModule,
}:

buildGoModule (finalAttrs: {
  pname = "scid";
  version = "git";

  src = lib.cleanSourceWith {
    filter =
      name: type:
      lib.cleanSourceFilter name type
      && !(builtins.elem (baseNameOf name) [
        "nix"
        "flake.nix"
      ]);

    src = ../.;
  };

  vendorHash = "sha256-8wlSYD4RvbG2L5tTq/xkUJGM3ILKdNKCP+TqGa9bsqo=";

  meta = {
    description = "Your frenly neighbourhood CI/CD.";
    homepage = "https://github.com/sinanmohd/scid";
    platforms = lib.platforms.unix;
    license = lib.licenses.agpl3Plus;
    mainProgram = "scid";
    maintainers = with lib.maintainers; [ sinanmohd ];
  };
})
