{
  lib,
  scid,
  dockerTools,
}:
dockerTools.buildLayeredImage {
  name = "sinanmohd/scid";
  tag = "git";

  contents = [ scid ];

  config.Cmd = [
    (lib.getExe scid)
  ];
}
