# ROM de un solo usuario y permisos por prefijo

La semántica de `java.io.File` se evalúa sobre un árbol ROM en memoria, fijo, con un único almacenamiento;
no se accede al FS del dispositivo ni del navegador. El almacenamiento es `/storage/emulated/0` y `/sdcard`
es solo un alias suyo. Opcionalmente el árbol queda persistido en OPFS mediante `--guardar-rom`, nunca en el
FS del aparato: en la pestaña el `fs` de `hospedador.js` ya es OPFS, así que `os.WriteFile` del intérprete
escribe ahí; sin la bandera no se escribe nada. En el ejecutable nativo la misma bandera escribe el fichero
que se le pida, fuera del árbol de la ROM, y sigue sin tocar los ficheros que la app pide desde Java.

Cuando la app pide un permiso, el emulador registra la petición y devuelve la denegación por defecto del
modelo; no hay diálogo del sistema ni cambio de permisos reales. La petición se anota en `ejecucion.avisos`
(`previewinterprete.go:213`); no se inventa un campo JSON nuevo. `checkSelfPermission` devuelve
`PERMISSION_DENIED` (-1) salvo que la sesión haya concedido el permiso con `--permiso`. Un permiso concedido
sigue sin dar acceso a todo el árbol: `accesoPermitido` aplica la regla de prefijo más larga y, si ninguna
regla casa, deniega. Con `targetSdk <= 29`, `READ_EXTERNAL_STORAGE` se considera implícito cuando se concedió
`WRITE_EXTERNAL_STORAGE`, como en el almacenamiento legacy.

## Permisos medidos del corpus

Fuente: `~/.cache/apkc-web/permisos-corpus/` (volcados de `apkanalyzer manifest print` y `aapt2 dump
badging` de las cinco APK de `docs/evidencia/15-apps-reales.md`). Las decisiones son las de esta ROM, no un
juicio sobre las apps.

| APK | minSdk | targetSdk | Permisos de almacenamiento e instalación declarados | Decisión |
| --- | --- | --- | --- | --- |
| Gestor (`com.martinmimigames.simplefileexplorer`, «Little File Explorer») | 1 | 29 | `WRITE_EXTERNAL_STORAGE`, `READ_EXTERNAL_STORAGE`, `MANAGE_EXTERNAL_STORAGE`, `REQUEST_INSTALL_PACKAGES` | Conceder `READ` y `WRITE` solo con `--permiso`, bajo `/storage/emulated/0` (`WRITE` hasta `/Download` por regla más larga); denegar `MANAGE_EXTERNAL_STORAGE` y `REQUEST_INSTALL_PACKAGES` siempre |
| F-Droid | 24 | 37 | `MANAGE_EXTERNAL_STORAGE`, `READ_EXTERNAL_STORAGE`, `WRITE_EXTERNAL_STORAGE`, `REQUEST_INSTALL_PACKAGES` y otros | Denegar por defecto; aunque se conceda `WRITE`, `targetSdk 37` no implica `READ` y `MANAGE` queda denegado |
| NewPipe | 23 | 35 | `WRITE_EXTERNAL_STORAGE` (`READ_EXTERNAL_STORAGE` implícito) | Denegar por defecto; no hay `MANAGE`; con `--permiso WRITE` se permite solo lo que la regla de prefijo concede |
| Termux | 24 | 28 | `MANAGE_EXTERNAL_STORAGE`, `WRITE_EXTERNAL_STORAGE`, `REQUEST_INSTALL_PACKAGES` (`READ_EXTERNAL_STORAGE` implícito) | Denegar `MANAGE` y `REQUEST_INSTALL_PACKAGES`; con `--permiso WRITE`, `targetSdk 28` da `READ` implícito |
| Libre Memory Game | 24 | 35 | Ninguno | Denegar todo: no hay permiso declarado que conceder |

`MANAGE_EXTERNAL_STORAGE` aparece en 3 de 5 APK (gestor, F-Droid y Termux) y se deniega en las tres: el
modelo no tiene un único usuario con acceso total, sino un árbol fijo con permisos por prefijo.

## Banderas

- `--rom fichero.json` carga otra ROM en lugar de `rom.json` (que viaja dentro del binario como
  `romPorDefectoJSON` y se comprueba contra el fichero en `TestRomElFicheroCoincideConElValorSembrado`).
- `--guardar-rom fichero.json` escribe el árbol resultante al terminar. Por defecto está desactivada.
- `--permiso android.permission.…` concede un permiso a la sesión; es repetible. No concede
  `MANAGE_EXTERNAL_STORAGE` ni `REQUEST_INSTALL_PACKAGES` aunque se pida: no hay reglas que lo permitan.
