package main

// La fidelidad visual de la Fase E4: lo que el modelo tiene que decir para que el dibujo se parezca
// al diseño. Se arma un apk en memoria con los escritores del fork (regla 2) y se comprueba el JSON
// que sale, no solo la estructura en Go:
//   - un TextView `gone` no ocupa sitio ni con su margen; el siguiente empieza en el origen;
//   - un TextView con `textSize` de 24 sp escala su alto y el ancho de cada carácter;
//   - `textColor` por `@color/x` de la tabla y `textStyle`/`background` por un `style="@style/x"`;
//   - un `background="#rrggbb"` escrito en el layout;
//   - un ImageView con un PNG propio del apk, con sus píxeles decodificados en Go;
//   - `layout_gravity="end"` coloca en el borde derecho.
//
// La geometría no tiene oráculo de aparato en esta máquina: lo que se fija es la cuenta declarada
// (tamaño del texto proporcional al declarado, media línea de 1,5 el tamaño) y los rectángulos que
// salen de esa cuenta.

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// pngDeDosPorDos arma un PNG de 2x2 con cuatro colores distintos en orden de filas: rojo, verde,
// azul y blanco. Es el mínimo que distingue una imagen decodificada de una caja vacía.
func pngDeDosPorDos(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	img.Set(1, 0, color.RGBA{G: 0xff, A: 0xff})
	img.Set(0, 1, color.RGBA{B: 0xff, A: 0xff})
	img.Set(1, 1, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		t.Fatalf("el PNG de prueba: %v", err)
	}
	return buf.Bytes()
}

// tablaVisual es la tabla del apk de la prueba: color propio, drawable con el PNG, id, layout y un
// estilo propio con textStyle y background. Los ids salen del orden alfabético de los tipos, como
// los numera aapt2: color 1, drawable 2, id 3, layout 4, style 5.
func tablaVisual() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.apkcstudio",
		global:        []string{"res/layout/main.xml", "res/drawable/punto.png"},
		globalUTF8:    true,
		nombresTipo:   []string{"color", "drawable", "id", "layout", "style"},
		nombresUTF8:   false,
		claves:        []string{"acento", "punto", "raiz", "main", "Titulo"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 0, valor: valorEscribir{tipo: tipoColor, datos: 0xff336699}},
			}}}},
			{id: 2, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 1, valor: valorEscribir{tipo: tipoCadena, datos: 1}},
			}}}},
			{id: 3, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: tipoBooleano, datos: 0}},
			}}}},
			{id: 4, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 3, valor: valorEscribir{tipo: tipoCadena, datos: 0}},
			}}}},
			{id: 5, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{
					clave:    4,
					flags:    0x0001,
					compleja: true,
					registros: []registroEscribir{
						{nombre: idVistaTextStyle, valor: valorEscribir{tipo: tipoEnteroDec, datos: 1}},
						{nombre: idVistaBackground, valor: valorEscribir{tipo: tipoColor, datos: 0xff102030}},
					},
				},
			}}}},
		},
	}
}

// layoutVisual es el layout de la prueba: un `gone`, el TextView con tamaño y color, el ImageView y
// un `invisible` con layout_gravity="end" y fondo propio.
func layoutVisual() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: idAttrOrientation, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xffffffff},
						atributoDim("layout_height", idAttrLayoutHeight, tipoDimension, 0x00002801, ""),
						{ns: "android", nombre: "visibility", id: idVistaVisibility, tipo: tipoEnteroDec, datos: 8},
						atributoDim("layout_marginBottom", idAttrLayoutMarginAb, tipoDimension, 0x00000801, ""),
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Oculto"},
					},
				},
				{
					nombre: "TextView",
					linea:  16,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						atributoDim("textSize", idVistaTextSize, tipoDimension, 0x00001802, ""),
						{ns: "android", nombre: "textColor", id: idVistaTextColor, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "gravity", id: idVistaGravity, tipo: tipoCadena, texto: "center_vertical|end"},
						{ns: "", nombre: "style", tipo: tipoReferencia, datos: 0x7f050000},
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Aviso"},
					},
				},
				{
					nombre: "ImageView",
					linea:  24,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "src", id: idVistaSrc, tipo: tipoReferencia, datos: 0x7f020000},
					},
				},
				{
					nombre: "TextView",
					linea:  32,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "layout_width", id: idAttrLayoutWidth, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: idAttrLayoutHeight, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "visibility", id: idVistaVisibility, tipo: tipoEnteroDec, datos: 4},
						{ns: "android", nombre: "layout_gravity", id: idAttrLayoutGravity, tipo: tipoEnteroHex, datos: 0x00800005},
						{ns: "android", nombre: "background", id: idVistaBackground, tipo: tipoCadena, texto: "#00ff00"},
						{ns: "android", nombre: "text", id: idAttrText, tipo: tipoCadena, texto: "Fantasma"},
					},
				},
			},
		},
	}
}

// TestElPreviewRespetaVisibilidadTextoColorEImagen es el criterio de aceptación de E4: el JSON del
// modelo con rectángulos, el `gone` sin ocupar espacio, el texto con su tamaño y color y el nodo
// `imagen` con los cuatro píxeles del PNG.
func TestElPreviewRespetaVisibilidadTextoColorEImagen(t *testing.T) {
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutVisual())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaVisual())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml":    manifiesto,
		"resources.arsc":         arsc,
		"res/layout/main.xml":    layout,
		"res/drawable/punto.png": pngDeDosPorDos(t),
	})

	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk de la prueba no se deja leer: %v", err)
	}
	raiz := m.Raiz
	if raiz == nil || len(raiz.Hijos) != 4 {
		t.Fatalf("la raíz trae %d hijos y son cuatro", len(raiz.Hijos))
	}
	oculto, aviso, imagenView, invisible := raiz.Hijos[0], raiz.Hijos[1], raiz.Hijos[2], raiz.Hijos[3]

	// GONE: rectángulo a cero y sus 40 dp de alto y 8 de margen no desplazan a nadie
	if oculto.Rect != (rectVista{}) {
		t.Errorf("el TextView gone ocupa %+v: no debería ocupar nada", oculto.Rect)
	}
	if aviso.Rect.Y != 0 {
		t.Errorf("el TextView visible empieza en y=%d: el gone ocupó espacio", aviso.Rect.Y)
	}
	// 5 caracteres x 12 dp (24 sp / 16 de referencia x 8) + 16 de relleno = 76; alto 24 x 1,5 = 36
	if aviso.Rect != (rectVista{X: 0, Y: 0, Ancho: 76, Alto: 36}) {
		t.Errorf("el TextView de 24 sp se coloca en %+v: se esperaba {0 0 76 36}", aviso.Rect)
	}
	if imagenView.Rect != (rectVista{X: 0, Y: 36, Ancho: 2, Alto: 2}) {
		t.Errorf("el ImageView se coloca en %+v: se esperaba {0 36 2 2}", imagenView.Rect)
	}
	// el invisible sí ocupa: 8 caracteres x 8 + 16 = 80, con layout_gravity end va a 360-80
	if invisible.Rect != (rectVista{X: 280, Y: 38, Ancho: 80, Alto: 24}) {
		t.Errorf("el TextView invisible se coloca en %+v: se esperaba {280 38 80 24}", invisible.Rect)
	}

	if aviso.TamTexto != "24sp" || aviso.Color != "#336699" {
		t.Errorf("el texto dice tamTexto=%q color=%q", aviso.TamTexto, aviso.Color)
	}
	if aviso.EstiloTexto != "bold" || aviso.Fondo != "#102030" {
		t.Errorf("el estilo propio no se resolvió: estiloTexto=%q fondo=%q", aviso.EstiloTexto, aviso.Fondo)
	}
	if invisible.Fondo != "#00ff00" {
		t.Errorf("el background escrito en el layout se lee como %q", invisible.Fondo)
	}
	if imagenView.Imagen == nil || imagenView.Imagen.Ancho != 2 || imagenView.Imagen.Alto != 2 {
		t.Fatalf("el ImageView no trae su imagen: %+v", imagenView.Imagen)
	}
	// `gravity` fino en una hoja: las dos mitades se leen de la misma tabla de bits
	if !strings.Contains(aviso.Gravity, "center_vertical") || !strings.Contains(aviso.Gravity, "end") {
		t.Errorf("gravity=\"center_vertical|end\" se lee como %q", aviso.Gravity)
	}

	// y lo mismo en el JSON, que es lo que consume la pestaña
	salida, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("el modelo no se serializa: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(salida, &doc); err != nil {
		t.Fatalf("el JSON del modelo no se relee: %v", err)
	}
	hijos := doc["raiz"].(map[string]any)["hijos"].([]any)
	goneJSON := hijos[0].(map[string]any)
	if goneJSON["visibilidad"] != "gone" {
		t.Errorf("el JSON del gone dice visibilidad=%v", goneJSON["visibilidad"])
	}
	rectGone := goneJSON["rect"].(map[string]any)
	if rectGone["ancho"].(float64) != 0 || rectGone["alto"].(float64) != 0 {
		t.Errorf("el JSON del gone lleva rectángulo %v", rectGone)
	}
	avisoJSON := hijos[1].(map[string]any)
	for campo, esperado := range map[string]string{
		"tamTexto": "24sp", "color": "#336699", "estiloTexto": "bold", "fondo": "#102030",
	} {
		if avisoJSON[campo] != esperado {
			t.Errorf("el JSON del aviso trae %s=%v: se esperaba %q", campo, avisoJSON[campo], esperado)
		}
	}
	imagenJSON, ok := hijos[2].(map[string]any)["imagen"].(map[string]any)
	if !ok {
		t.Fatalf("el JSON del ImageView no trae nodo imagen: %v", hijos[2])
	}
	pixeles, ok := imagenJSON["pixeles"].([]any)
	if !ok || len(pixeles) != 4 {
		t.Fatalf("la imagen del JSON trae %v píxeles y son cuatro", imagenJSON["pixeles"])
	}
	for i, esperado := range []string{"#ff0000ff", "#00ff00ff", "#0000ffff", "#ffffffff"} {
		if pixeles[i] != esperado {
			t.Errorf("el píxel %d es %v: se esperaba %q", i, pixeles[i], esperado)
		}
	}
	if imagenJSON["ancho"].(float64) != 2 || imagenJSON["alto"].(float64) != 2 {
		t.Errorf("la imagen del JSON mide %vx%v", imagenJSON["ancho"], imagenJSON["alto"])
	}
}

// TestElColorRGB8DeAapt2SeLeeComoColor cierra la condición H4 del crítico de E4: aapt2 escribe
// TYPE_INT_COLOR_RGB8 (0x1d) para un `#rrggbb` de res/values/colors.xml, no ARGB8 (0x1c).
// Medido con `aapt2 dump resources` sobre un proyecto con `<color name="probe">#336699</color>`.
func TestElColorRGB8DeAapt2SeLeeComoColor(t *testing.T) {
	c := &contextoVista{}
	valor := valor{tipo: 0x1d, datos: 0xff336699}
	if obtenido := c.colorDe(valor); obtenido != "#336699" {
		t.Fatalf("colorDe con tipo 0x1d dio %q: se esperaba %q", obtenido, "#336699")
	}
}

// La resolución del tema de la Fase E se comprueba con el mismo APK del adaptador: el manifiesto
// elige un estilo del paquete y el modelo tiene que publicar su texto y su fondo en los campos que
// ya existen (`color` y `fondo`), también en las filas que devuelve el adaptador. El nombre del
// estilo se eligió con prefijo ofuscado a propósito: la resolución va por id de atributo.

// tablaDeListasConTema añade un color y un estilo al final de la tabla de listas, sin mover los ids
// de `id` y `layout` que las pruebas anteriores ya usan.
func tablaDeListasConTema() tablaEscribir {
	t := tablaDeListas()
	t.nombresTipo = append(append([]string{}, t.nombresTipo...), "color", "style")
	t.claves = append(append([]string{}, t.claves...), "tinta", "abc_TemaOfuscado")
	t.tipos = append(append([]tipoEscribir{}, t.tipos...),
		tipoEscribir{id: 3, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
			{clave: 5, valor: valorEscribir{tipo: tipoColor, datos: 0xff123456}},
		}}}},
		tipoEscribir{id: 4, specFlags: []uint32{0}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
			{clave: 6, flags: 0x0001, compleja: true, registros: []registroEscribir{
				{nombre: idAtributoTextoTema, valor: valorEscribir{tipo: tipoReferencia, datos: 0x7f030000}},
				{nombre: idAtributoFondoVentana, valor: valorEscribir{tipo: tipoReferencia, datos: idColorBlanco}},
			}},
		}}}},
	)
	return t
}

func manifiestoDeListasConTema() documentoAXML {
	m := manifiestoDeListas()
	aplicacion := m.raiz.hijos[0]
	aplicacion.attrs = append(aplicacion.attrs, atributoEscribir{
		ns: "android", nombre: "theme", id: idAtributoTema,
		tipo: tipoReferencia, datos: 0x7f040000,
	})
	return m
}

func apkDeListasConTema(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDeListasConTema())
	if err != nil {
		t.Fatalf("el manifiesto con tema: %v", err)
	}
	main, err := escribeAXML(layoutDeLaLista())
	if err != nil {
		t.Fatalf("el layout main: %v", err)
	}
	fila, err := escribeAXML(layoutDeLaFila())
	if err != nil {
		t.Fatalf("el layout fila: %v", err)
	}
	tabla := tablaDeListasConTema()
	arsc, err := escribeTablaRecursos(tabla)
	if err != nil {
		t.Fatalf("la tabla con tema: %v", err)
	}
	r, err := rDeTabla(tabla)
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": main,
		"res/layout/fila.xml": fila,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteMainListas},
			fuenteJava{nombre: "Adaptador.java", contenido: fuenteAdaptador}),
	})
}

// TestElModeloTraeElTemaYLasFilasDelAdaptador es el criterio falsable de la Fase E del lado Go: el
// color del texto del tema (una referencia a un `@color` ofuscado) y el fondo windowBackground
// (una referencia a un color del framework) llegan a los nodos, y las filas del adaptador traen su
// texto real. No se añade ninguna clave al JSON.
func TestElModeloTraeElTemaYLasFilasDelAdaptador(t *testing.T) {
	modelo, err := modeloVistaDeAPKConPulsos(apkDeListasConTema(t), "", true, nil)
	if err != nil {
		t.Fatalf("el apk con tema no se lee: %v", err)
	}
	if modelo.Raiz == nil {
		t.Fatal("el modelo no trae raíz")
	}
	if modelo.Raiz.Fondo != "#ffffff" {
		t.Errorf("el fondo del tema no llegó a la raíz: %q", modelo.Raiz.Fondo)
	}
	if len(modelo.Raiz.Hijos) != 2 {
		t.Fatalf("la pantalla no tiene sus dos hijos: %+v", modelo.Raiz.Hijos)
	}
	if pie := modelo.Raiz.Hijos[1]; pie.Color != "#123456" {
		t.Errorf("el color del tema no llegó al TextView: %q", pie.Color)
	}
	lista := modelo.Raiz.Hijos[0]
	if lista.Tipo != "ListView" || len(lista.Hijos) != 3 {
		t.Fatalf("la lista no trae tres filas: %s, %+v", lista.Tipo, lista.Hijos)
	}
	esperados := []string{"Fila 0: Alfa", "Fila 1: Beta", "Fila 2: Gama"}
	for i, fila := range lista.Hijos {
		if texto := textoDeLaFila(fila); texto != esperados[i] {
			t.Errorf("la fila %d dice %q y se esperaba %q", i, texto, esperados[i])
		}
		if fila.Color != "#123456" {
			t.Errorf("la fila %d no heredó el color del tema: %q", i, fila.Color)
		}
	}
	crudo, err := json.Marshal(modelo)
	if err != nil {
		t.Fatalf("el modelo no se serializa: %v", err)
	}
	if strings.Contains(string(crudo), `"tema"`) {
		t.Error("el JSON añadió un campo `tema`: el criterio exige reutilizar color/fondo")
	}
}
