# i41eu-assets

Assets for the Stackchan projects: robot3d (a 3D model of the robot drawn in Go, with any screen).

- Go only, standard library only (robot3d is imported by other tools, e.g. a video recorder).
- Photos from a phone carry GPS: never commit them as they are (`photos-*/` is ignored).
- Never name things "stackchan-..." (trademark); labels on the model are drawn without text.
- robot3d/m5stack/*.stl are M5Stack's files (MIT), unchanged; keep their LICENSE next to them.
- Check a change against the photos: render the same view and put them side by side.
- `GOWORK=off go test ./...` (the parent directory has a go.work without this module).
- The robot pictures in other repos are written by robot3d-distribute: change renders.json or
  screens/ here, rerender, distribute, then commit in each repo; never edit those copies by hand.
- screens/ must not show private data (LAN addresses, real device ids, hostnames), also not
  inside a QR code: e.g. the Manager screen with a LAN address is kept out of renders.json, and
  screens/qr.png's code is a placeholder (https://raw.q41.eu/pair?code=T4JCX68Y#e2e=1.AAAA….AAAA…,
  drawn over the real screen's code, same version and size). Decode a screen's QR code before
  adding it.
