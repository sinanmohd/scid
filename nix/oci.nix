{
  lib,
  pkgs,
  scid,
  dockerTools,
}:
dockerTools.buildLayeredImage {
  name = "sinanmohd/scid";
  tag = "git";

  contents = [
    scid
    pkgs.dockerTools.caCertificates
  ];

  config = {
    WorkingDir = "/var/lib/scid";
    Cmd = [
      (lib.getExe scid)
    ];
  };
}
