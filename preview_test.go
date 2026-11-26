package main

// Las pruebas de la Fase 1 de la vista previa. Hay dos clases, y las dos hacen falta:
//   - simetría: lo que escriben axmlwriter.go y arscwriter.go se lee con el lector y se resuelve
//     igual que lo escrito (mismos nombres, mismas cadenas, mismos ids);
//   - los bytes reales de aapt2 que ya fijó la Fase 3 en base64 (axmlManifiesto, axmlLayout y
//     arscTabla), que son el otro extremo: el lector tiene que servir para el fichero de una
//     herramienta que no es este fork.
//
// El apk se arma en memoria con archive/zip: no se versiona ningún binario (regla 2), y así la
// prueba del comando completo no depende de un sdk ni de un proceso.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zipDeEntradas arma un zip en memoria con las entradas dadas, que es lo que espera un lector de
// apk. Los nombres son los del paquete: AndroidManifest.xml, resources.arsc y res/layout/*.xml.
func zipDeEntradas(t *testing.T, entradas map[string][]byte) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for nombre, datos := range entradas {
		w, err := zw.Create(nombre)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if _, err := w.Write(datos); err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// apkDeLosEscritores es el apk completo salido del fork: el manifiesto y el layout de
// axmlwriter_test.go y la tabla de arscwriter_test.go, que son los mismos bytes que aapt2 deja en
// el proyecto del studio. Nada de aquí lo escribió aapt2: es la simetría de punta a punta.
func apkDeLosEscritores(t *testing.T) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelStudio())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
	})
}

// TestPreviewLeeElAXMLQueEscribeElFork: el árbol del layout, con sus atributos y el texto que el
// escritor dejó en el pool del propio fichero. Sin tabla, un id se queda en su número.
func TestPreviewLeeElAXMLQueEscribeElFork(t *testing.T) {
	datos, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	a, err := leeAXML(datos)
	if err != nil {
		t.Fatalf("el AXML del fork no se deja leer: %v", err)
	}
	if !a.tabla.utf8 {
		t.Error("el layout se escribe en UTF-8 y el lector dice UTF-16")
	}
	ctx := &contextoVista{axml: a}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol: %v", err)
	}
	if arbol.nombre != "LinearLayout" {
		t.Fatalf("la raíz se lee como %q", arbol.nombre)
	}
	if len(arbol.hijos) != 2 || arbol.hijos[0].nombre != "TextView" || arbol.hijos[1].nombre != "Button" {
		t.Fatalf("los hijos se leen como %d nodos, y se esperaban TextView y Button", len(arbol.hijos))
	}
	if texto := ctx.textoDeAtributo(arbol.hijos[0], uriDeRecursos, "text"); texto != "Hello World!" {
		t.Errorf("el texto del TextView se lee como %q", texto)
	}
	if texto := ctx.textoDeAtributo(arbol.hijos[1], uriDeRecursos, "text"); texto != "Press me" {
		t.Errorf("el texto del Button se lee como %q", texto)
	}
	// el ancho va como entero con signo: -2 es wrap_content y -1 match_parent
	ancho, ok := arbol.hijos[0].atributo(uriDeRecursos, "layout_width")
	if !ok || valorDeMedida(ancho) != "wrap_content" {
		t.Errorf("layout_width del TextView: %+v -> %q", ancho, valorDeMedida(ancho))
	}
	alto, ok := arbol.atributo(uriDeRecursos, "layout_height")
	if !ok || valorDeMedida(alto) != "match_parent" {
		t.Errorf("layout_height de la raíz: %+v -> %q", alto, valorDeMedida(alto))
	}
	// sin tabla, la referencia del id no tiene nombre y se queda su número
	id, ok := arbol.hijos[1].atributo(uriDeRecursos, "id")
	if !ok || ctx.texto(id) != "@0x7f010000" {
		t.Errorf("el id sin tabla se lee como %q, y el fichero guarda la referencia 0x7f010000", ctx.texto(id))
	}
	// la orientación la escribió el enlace como el enum 1, que es vertical
	orientacion, _ := arbol.atributo(uriDeRecursos, "orientation")
	if int32(orientacion.datos) != 1 {
		t.Errorf("orientation se lee como %d, el fichero guarda 1 (vertical)", int32(orientacion.datos))
	}
}

// TestPreviewResuelveLaTablaQueEscribeElFork: los ids de un layout, contra la tabla del propio
// escritor. Incluye la segunda configuración con una clave ausente, que hasta ahora el lector no
// sabía leer: el offset 0xffffffff se guarda como hueco en su sitio y no revienta.
func TestPreviewResuelveLaTablaQueEscribeElFork(t *testing.T) {
	tb := tablaDelStudio()
	// una `values-es` que no declara `main`: es la forma medida en el §94, con el offset -1
	tb.tipos[1].bloques = append(tb.tipos[1].bloques, bloqueEscribir{
		config:   "es",
		entradas: []entradaEscribir{{clave: 1, falta: true}},
	})
	datos, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	tr, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla del fork no se deja leer: %v", err)
	}

	tipo, clave, ok := tr.nombreDeId(0x7f010000)
	if !ok || tipo != "id" || clave != "pulsar" {
		t.Errorf("0x7f010000 resuelve a %q/%q (ok=%v), y la tabla lo llama id/pulsar", tipo, clave, ok)
	}
	v, ok := tr.valorDeId(0x7f020000)
	if !ok || v.tipo != tipoCadena || int(v.datos) >= len(tr.global.cadenas) ||
		tr.global.cadenas[v.datos] != "res/layout/main.xml" {
		t.Errorf("0x7f020000 no apunta a la ruta del layout: %+v (ok=%v)", v, ok)
	}
	if _, ok := tr.valorDeId(0x7f030000); ok {
		t.Error("un estilo es un valor complejo y no tiene Res_value simple")
	}
	if _, _, ok := tr.nombreDeId(0x7f040000); ok {
		t.Error("un tipo que la tabla no tiene no puede resolver")
	}
	if tipo, clave, ok := tr.nombreDeId(0x01030042); ok {
		t.Errorf("un id del framework se resolvió como %q/%q sin estar en la tabla", tipo, clave)
	}

	ausentes := 0
	for _, bl := range tr.bloques {
		for _, e := range bl.entradas {
			if e.ausente {
				ausentes++
			}
		}
	}
	if ausentes != 1 {
		t.Errorf("la configuración `es` deja %d entradas ausentes y se esperaba una", ausentes)
	}
}

// TestPreviewSobreUnAPKDeLosEscritores: el comando completo sobre el apk que arma el propio fork.
// Es la prueba que dice que el modelo no depende de aapt2: manifiesto, tabla y layout salen de los
// escritores, y el lector los vuelve a juntar.
func TestPreviewSobreUnAPKDeLosEscritores(t *testing.T) {
	m, err := modeloVistaDeAPK(apkDeLosEscritores(t), "")
	if err != nil {
		t.Fatalf("el apk del fork no se deja leer: %v", err)
	}
	if m.Paquete != "com.hola.apkcstudio" {
		t.Errorf("el paquete se lee como %q", m.Paquete)
	}
	if m.Version != "0.1" {
		t.Errorf("la versión se lee como %q y el manifiesto dice 0.1", m.Version)
	}
	if m.Actividad != ".MainActivity" {
		t.Errorf("la actividad lanzadora se lee como %q", m.Actividad)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("el layout se lee como %q", m.Layout)
	}
	if !m.Aproximado || m.Pantalla.Ancho != dpPantallaAncho || m.Pantalla.Alto != dpPantallaAlto {
		t.Errorf("la pantalla del modelo es %+v (aproximado=%v)", m.Pantalla, m.Aproximado)
	}
	if m.Nodos != 3 || m.Raiz == nil || m.Raiz.Tipo != "LinearLayout" {
		t.Fatalf("el modelo trae %d nodos con raíz %+v", m.Nodos, m.Raiz)
	}
	if r := m.Raiz.Rect; r.X != 0 || r.Y != 0 || r.Ancho != dpPantallaAncho || r.Alto != dpPantallaAlto {
		t.Errorf("la raíz ocupa %+v y el layout pide match_parentxmatch_parent", r)
	}
	if len(m.Raiz.Hijos) != 2 {
		t.Fatalf("la raíz trae %d hijos", len(m.Raiz.Hijos))
	}
	texto, boton := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if texto.Tipo != "TextView" || texto.Texto != "Hello World!" {
		t.Errorf("el primer hijo es %q con texto %q", texto.Tipo, texto.Texto)
	}
	if boton.Tipo != "Button" || boton.Texto != "Press me" {
		t.Errorf("el segundo hijo es %q con texto %q", boton.Tipo, boton.Texto)
	}
	if boton.ID != "id/pulsar" || boton.IDRef != "0x7f010000" {
		t.Errorf("el id del Button se lee como %q (%q), y la tabla lo llama id/pulsar", boton.ID, boton.IDRef)
	}
	if texto.Ancho != "wrap_content" || texto.Alto != "wrap_content" {
		t.Errorf("el TextView declara %s x %s", texto.Ancho, texto.Alto)
	}
	// el TextView y el Button se centran los dos: el layout declara gravity="center" y el bloque de
	// hijos queda en el medio de la pantalla (640 - 24 - 48 = 568, entre 2 = 284)
	if texto.Rect.Y != 284 || boton.Rect.Y != texto.Rect.Y+texto.Rect.Alto {
		t.Errorf("con gravity=center se esperaba el TextView en y=284 y el Button en y=%d; están en %d y %d",
			texto.Rect.Y+texto.Rect.Alto, texto.Rect.Y, boton.Rect.Y)
	}
	if m.Raiz.Gravity != "center" {
		t.Errorf("la raíz declara gravity=center y el modelo dice %q", m.Raiz.Gravity)
	}
}

// TestPreviewSobreLosBytesDeAapt2: los tres ficheros reales de la Fase 3, los que escribió el aapt2
// del teléfono sobre el proyecto del studio, metidos en un apk en memoria. Es el otro extremo de la
// simetría y la prueba de que el formato leído no es solo el que escribimos nosotros.
func TestPreviewSobreLosBytesDeAapt2(t *testing.T) {
	manifiesto := mustBase64(t, axmlManifiesto, 2000)
	layout := mustBase64(t, axmlLayout, 784)
	arsc := mustBase64(t, arscTabla, 1088)
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
	})
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk de aapt2 no se deja leer: %v", err)
	}
	if m.Paquete != "com.hola.apkcstudio" || m.Version != "0.1" || m.Actividad != ".MainActivity" {
		t.Errorf("el manifiesto de aapt2 se lee como %q %q %q", m.Paquete, m.Version, m.Actividad)
	}
	if m.Layout != "res/layout/main.xml" || m.Nodos != 3 || m.Raiz == nil {
		t.Fatalf("layout %q con %d nodos y raíz %+v", m.Layout, m.Nodos, m.Raiz)
	}
	if m.Raiz.Hijos[0].Texto != "Hello World!" || m.Raiz.Hijos[1].Texto != "Press me" {
		t.Errorf("los textos se leen como %q y %q", m.Raiz.Hijos[0].Texto, m.Raiz.Hijos[1].Texto)
	}
	if m.Raiz.Hijos[1].ID != "id/pulsar" {
		t.Errorf("el id del Button se lee como %q", m.Raiz.Hijos[1].ID)
	}
	if m.Raiz.Hijos[0].Estilo != "@0x01030042" {
		t.Errorf("el style del TextView es un id del framework y se lee como %q", m.Raiz.Hijos[0].Estilo)
	}
}

// TestPreviewRechazaLoQueNoEs: los errores van dichos por su nombre, que en la pestaña es lo único
// que el usuario llega a ver cuando el paquete está a medias.
func TestPreviewRechazaLoQueNoEs(t *testing.T) {
	if _, err := modeloVistaDeAPK([]byte("no soy un zip"), ""); err == nil {
		t.Error("se aceptó un fichero que no es un zip")
	}
	sinManifiesto := zipDeEntradas(t, map[string][]byte{"res/layout/main.xml": {0x03, 0x00, 0x08, 0x00}})
	if _, err := modeloVistaDeAPK(sinManifiesto, ""); err == nil {
		t.Error("se aceptó un apk sin AndroidManifest.xml")
	}
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	sinLayout := zipDeEntradas(t, map[string][]byte{"AndroidManifest.xml": manifiesto})
	if m, err := modeloVistaDeAPK(sinLayout, ""); err != nil || m.Layout != "" ||
		m.Ejecucion == nil || len(m.Ejecucion.Avisos) != 1 ||
		!strings.HasPrefix(m.Ejecucion.Avisos[0], avisoSinLayout) {
		t.Errorf("un apk sin layout no publicó el centinela: err %v, modelo %+v", err, m)
	}
	if _, err := modeloVistaDeAPK(apkDeLosEscritores(t), "otro"); err == nil {
		t.Error("se aceptó pedir un layout que la tabla no tiene")
	}
	// y el layout pedido por nombre sí se encuentra
	if m, err := modeloVistaDeAPK(apkDeLosEscritores(t), "main"); err != nil || m.Layout != "res/layout/main.xml" {
		t.Errorf("pedir el layout main: %v, %q", err, m.Layout)
	}
	// un paquete sin tabla: el layout se busca en el zip y las referencias se quedan en su número
	layout, err := escribeAXML(layoutDelStudio())
	if err != nil {
		t.Fatal(err)
	}
	sinTabla := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"res/layout/main.xml": layout,
	})
	m, err := modeloVistaDeAPK(sinTabla, "")
	if err != nil {
		t.Fatalf("un apk sin resources.arsc tiene que leerse igual: %v", err)
	}
	if m.Layout != "res/layout/main.xml" || m.Raiz.Hijos[1].ID != "@0x7f010000" {
		t.Errorf("sin tabla: layout %q e id %q", m.Layout, m.Raiz.Hijos[1].ID)
	}
}

func mustBase64(t *testing.T, s string, mide int) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	if len(b) != mide {
		t.Fatalf("el fixture mide %d bytes y se esperaban %d", len(b), mide)
	}
	return b
}

// Cadenas que no son ASCII: en el pool UTF-8 de Android el primer prefijo de una cadena cuenta
// unidades utf-16 y el segundo bytes, y con estos textos dejan de coincidir. «Hola & adiós ñ» mide
// 14 unidades y 16 bytes; el emoji, dos unidades y seis bytes, porque aapt2 lo escribe como su par
// sustituto en CESU-8 (3+3), no como los cuatro bytes del UTF-8 de Go.
const (
	textoAcentos = "Hola & adiós ñ"
	textoEmoji   = "hola \U0001F600"
	textoAmp     = "&amp;"
)

// poolAapt2Emoji son los 100 bytes del trozo RES_STRING_POOL (UTF-8) que aapt2 35.0.1 escribió en
// el resources.arsc de la sonda, copiados tal cual desde el offset 12 del fichero. Sus cadenas, en
// orden, son las de abajo: el «&amp;» que sobrevive al XML, «Hola & adiós ñ 😀» —con 17 unidades
// declaradas y 23 bytes, el emoji como par sustituto—, «ñ» y «😀» solo.
const poolAapt2Emoji = "AQAcAGQAAAAFAAAAAAAAAAABAAAwAAAAAAAAAAAAAAADAAAACwAAACUAAAAqAAAAAAAABQUmYW1wOwARF0hvbGEgJiBhZGnDs3Mgw7Eg7aC97biAAAECw7EAAgbtoL3tuIAAAA=="

// TestPreviewLeeElPoolQueEscribeAapt2: el pool UTF-8 tal cual lo dejó aapt2 35.0.1 en un
// resources.arsc con emoji. No es un ida y vuelta del escritor propio: los bytes están copiados
// del fichero de la herramienta, y son el caso que el lector rechazaba («claims 17 utf-16 units
// and its 23 bytes hold 21») porque la rama de UTF-8 copiaba los bytes sin decodificar el CESU-8.
// Además, el escritor tiene que reproducir ese trozo byte a byte.
func TestPreviewLeeElPoolQueEscribeAapt2(t *testing.T) {
	crudo := mustBase64(t, poolAapt2Emoji, 100)
	p, err := leePool(crudo, 0)
	if err != nil {
		t.Fatalf("el pool de aapt2 con emoji no se deja leer: %v", err)
	}
	esperadas := []string{"", "&amp;", "Hola & adiós ñ \U0001F600", "ñ", "\U0001F600"}
	if len(p.cadenas) != len(esperadas) {
		t.Fatalf("el pool devuelve %d cadenas y tiene %d", len(p.cadenas), len(esperadas))
	}
	for i, esperada := range esperadas {
		if p.cadenas[i] != esperada {
			t.Errorf("cadena #%d leída como %q (% x), escrita %q", i, p.cadenas[i], []rune(p.cadenas[i]), esperada)
		}
	}
	// el emoji vuelve como un rune U+1F600, no como las dos unidades sueltas del par sustituto
	if r := []rune(p.cadenas[4]); len(r) != 1 || r[0] != 0x1F600 {
		t.Errorf("el emoji se lee como %d runes: %U", len(r), r)
	}
	if !p.utf8 {
		t.Error("el trozo de aapt2 está en modo UTF-8 y el lector dice que no")
	}
	// y el escritor del fork vuelve a producir esos mismos 100 bytes
	if nuestro := escribePool(esperadas, true); !bytes.Equal(nuestro, crudo) {
		t.Errorf("el pool escrito mide %d bytes y el de aapt2 %d\nnuestro: % x\nsuyo:   % x",
			len(nuestro), len(crudo), nuestro, crudo)
	}
}

// TestPreviewLeeElPoolUTF8ConAcentosYEmoji ataca la causa aislada del fallo de la Fase 1: leeCadena
// tomaba el primer prefijo como bytes y rompía con cualquier carácter multibyte. El pool se prueba
// suelto y dentro de los dos contenedores que lo usan: un AXML (UTF-8 como un layout y UTF-16 como
// el manifiesto) y un resources.arsc.
func TestPreviewLeeElPoolUTF8ConAcentosYEmoji(t *testing.T) {
	// el pool suelto, con la forma del formato y la terminación en cero en el sitio que toca
	datosPool := escribePool([]string{"id", textoAcentos, textoEmoji, textoAmp}, true)
	p, err := leePool(datosPool, 0)
	if err != nil {
		t.Fatalf("el pool UTF-8 con acentos no se deja leer: %v", err)
	}
	esperadas := []string{"id", textoAcentos, textoEmoji, textoAmp}
	if len(p.cadenas) != len(esperadas) {
		t.Fatalf("el pool devuelve %d cadenas y se escribieron %d", len(p.cadenas), len(esperadas))
	}
	for i, esperada := range esperadas {
		if p.cadenas[i] != esperada {
			t.Errorf("cadena #%d leída como %q, escrita %q", i, p.cadenas[i], esperada)
		}
	}

	// el mismo pool dentro de un AXML UTF-8, el modo del layout
	doc := documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  1,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				{nombre: "TextView", linea: 2, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoAcentos}}},
				{nombre: "TextView", linea: 3, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoEmoji}}},
				{nombre: "TextView", linea: 4, attrs: []atributoEscribir{
					{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: textoAmp}}},
			},
		},
	}
	datosAXML, err := escribeAXML(doc)
	if err != nil {
		t.Fatalf("el AXML con acentos: %v", err)
	}
	axml, err := leeAXML(datosAXML)
	if err != nil {
		t.Fatalf("el AXML con acentos no se deja leer: %v", err)
	}
	ctx := &contextoVista{axml: axml}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol del AXML con acentos: %v", err)
	}
	if len(arbol.hijos) != 3 {
		t.Fatalf("el layout trae %d hijos y se escribieron 3", len(arbol.hijos))
	}
	for i, esperada := range []string{textoAcentos, textoEmoji, textoAmp} {
		if leida := ctx.textoDeAtributo(arbol.hijos[i], uriDeRecursos, "text"); leida != esperada {
			t.Errorf("el texto #%d se lee como %q y se escribió %q", i, leida, esperada)
		}
	}

	// y el modo UTF-16 del manifiesto, que tiene que seguir intacto: un prefijo y dos bytes por
	// unidad, también con caracteres fuera del plano base
	manifiesto := manifiestoDelStudio()
	manifiesto.raiz.hijos[1].attrs[0].texto = textoAcentos // application/label
	datosManifiesto, err := escribeAXML(manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto UTF-16 con acentos: %v", err)
	}
	axmlManifiesto, err := leeAXML(datosManifiesto)
	if err != nil {
		t.Fatalf("el manifiesto UTF-16 con acentos no se deja leer: %v", err)
	}
	ctxManifiesto := &contextoVista{axml: axmlManifiesto}
	arbolManifiesto, err := ctxManifiesto.arbol()
	if err != nil {
		t.Fatalf("el árbol del manifiesto: %v", err)
	}
	app := arbolManifiesto.hijo("application")
	if app == nil {
		t.Fatal("el manifiesto no trae application")
	}
	if leida := ctxManifiesto.textoDeAtributo(app, uriDeRecursos, "label"); leida != textoAcentos {
		t.Errorf("el label UTF-16 se lee como %q y se escribió %q", leida, textoAcentos)
	}

	// y dentro de un resources.arsc: el global y las claves son los dos pools UTF-8 de la tabla
	tb := tablaDelStudio()
	tb.global = append(tb.global, textoAcentos, textoEmoji)
	tb.claves = append(tb.claves, textoAcentos, textoEmoji)
	datosTabla, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla con acentos: %v", err)
	}
	tr, err := leeTabla(datosTabla)
	if err != nil {
		t.Fatalf("la tabla con acentos no se deja leer: %v", err)
	}
	if n := len(tr.global.cadenas); n != 3 || tr.global.cadenas[1] != textoAcentos || tr.global.cadenas[2] != textoEmoji {
		t.Errorf("el pool global lee %d cadenas: %q", n, tr.global.cadenas)
	}
	if n := len(tr.claves); n != 5 || tr.claves[3] != textoAcentos || tr.claves[4] != textoEmoji {
		t.Errorf("el pool de claves lee %d cadenas: %q", n, tr.claves)
	}
}

// apkDelStudioConTexto es el apk de los escritores con el texto del TextView y el label del
// manifiesto cambiados, y la misma cadena añadida al pool global de la tabla: los tres sitios por
// los que pasa una cadena de la aplicación.
func apkDelStudioConTexto(t *testing.T, texto string) []byte {
	t.Helper()
	manifiesto := manifiestoDelStudio()
	manifiesto.raiz.hijos[1].attrs[0].texto = texto // application/label
	manifiestoBytes, err := escribeAXML(manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout := layoutDelStudio()
	layout.raiz.hijos[0].attrs[3].texto = texto // TextView/android:text
	layoutBytes, err := escribeAXML(layout)
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	tb := tablaDelStudio()
	tb.global = append(tb.global, texto)
	arsc, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiestoBytes,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layoutBytes,
	})
}

// TestPreviewSobreUnAPKConTextoNoASCII: de extremo a extremo, el mismo camino que hace el comando.
// Con el lector que tomaba el primer prefijo como bytes, este apk —que se arma con los escritores
// del fork, no con aapt2— fallaba al leer la tabla con «does not end with a zero byte». No se
// ejecuta el binario: lo que devolvía rc 2 era este error, y el comando solo lo imprime.
func TestPreviewSobreUnAPKConTextoNoASCII(t *testing.T) {
	apk := apkDelStudioConTexto(t, textoAcentos)
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk con «%s» no se deja leer: %v", textoAcentos, err)
	}
	if m.Raiz == nil || len(m.Raiz.Hijos) != 2 {
		t.Fatalf("el modelo no trae los dos hijos del layout: %+v", m.Raiz)
	}
	if leida := m.Raiz.Hijos[0].Texto; leida != textoAcentos {
		t.Errorf("el TextView se lee como %q y se escribió %q", leida, textoAcentos)
	}
	if m.Paquete != "com.hola.apkcstudio" || m.Layout != "res/layout/main.xml" {
		t.Errorf("paquete %q y layout %q", m.Paquete, m.Layout)
	}
	// el JSON que imprime el comando también lleva el texto. `encoding/json` escapa el `&`, así
	// que se busca la parte acentuada, que es la que el lector partía.
	salida, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(salida), "adiós ñ") {
		t.Errorf("el JSON no trae el texto acentuado: %s", salida)
	}
}

// TestElPreviewConservaLosEspaciosDelTextoEscrito: `--escribir` entrega el texto tal cual se tecleó,
// con los espacios finales y los de en medio. Antes se recortaba el valor completo en el flag y el
// texto llegaba podado al intérprete; solo se recortan los espacios delante del id y los de
// `--avanzar`, que no son contenido.
func TestElPreviewConservaLosEspaciosDelTextoEscrito(t *testing.T) {
	set := flag.NewFlagSet("preview", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	guion := []eventoUsuario{}
	set.Var(guionDeEntrada{tipo: "escribir", eventos: &guion}, "escribir", "")
	set.Var(guionDeEntrada{tipo: "avanzar", eventos: &guion}, "avanzar", "")
	err := set.Parse([]string{
		"--escribir", "id/x/hola  ",
		"--escribir", "id/x/  hola  ",
		"--escribir", "id/x/a/b",
		"--escribir", "  id/x/hola",
		"--escribir", "id/",
		"--avanzar", " 1000 ",
	})
	if err != nil {
		t.Fatalf("el guion no se parsea: %v", err)
	}
	esperado := []eventoUsuario{
		{Tipo: "escribir", Dato: "id/x/hola  "},
		{Tipo: "escribir", Dato: "id/x/  hola  "},
		{Tipo: "escribir", Dato: "id/x/a/b"},
		{Tipo: "escribir", Dato: "id/x/hola"},
		{Tipo: "escribir", Dato: "id/"},
		{Tipo: "avanzar", Dato: "1000"},
	}
	if len(guion) != len(esperado) {
		t.Fatalf("el guion tiene %d pasos y se esperaban %d: %+v", len(guion), len(esperado), guion)
	}
	for i := range esperado {
		if guion[i].Tipo != esperado[i].Tipo || guion[i].Dato != esperado[i].Dato {
			t.Errorf("el paso %d es %+v y se esperaba %+v", i, guion[i], esperado[i])
		}
	}
	// `partirEscribir` entrega el texto sin podar: finales, internos, barras y el id sin texto. El
	// valor con espacios delante del id se recorta solo ahí y el texto sigue siendo «hola».
	textos := []string{"hola  ", "  hola  ", "a/b", "hola", ""}
	for i, esperadoTexto := range textos {
		_, texto := partirEscribir(guion[i].Dato)
		if texto != esperadoTexto {
			t.Errorf("--escribir %q entrega %q y se esperaba %q", guion[i].Dato, texto, esperadoTexto)
		}
	}
	// un valor hecho solo de espacios no es un id y se sigue rechazando al parsear
	if err := (guionDeEntrada{tipo: "escribir", eventos: &guion}).Set("   "); err == nil {
		t.Error("--escribir aceptó un valor hecho solo de espacios")
	}
}

// rutaDelFicheroRom apunta al fichero versionable desde el directorio donde corre la suite. El
// intérprete materializado vive en apkc/, así que el submódulo está un nivel por encima.
func rutaDelFicheroRom() string {
	return filepath.Join("..", "studio", "apkc-emulador", "rom.json")
}

// TestRomElFicheroCoincideConElValorSembrado evita que el rom.json versionable y la ROM que viaja
// dentro del binario se separen: el fichero es la documentación y la constante es lo que corre en
// la pestaña, donde no hay FS del que leer.
func TestRomElFicheroCoincideConElValorSembrado(t *testing.T) {
	datos, err := os.ReadFile(rutaDelFicheroRom())
	if err != nil {
		t.Fatalf("leer rom.json: %v", err)
	}
	if string(datos) != romPorDefectoJSON {
		t.Errorf("rom.json y romPorDefectoJSON difieren: el fichero tiene %d bytes y la ROM %d",
			len(datos), len(romPorDefectoJSON))
	}
	var j romJSON
	if err := json.Unmarshal([]byte(romPorDefectoJSON), &j); err != nil {
		t.Fatalf("la ROM sembrada no es JSON: %v", err)
	}
	if j.Almacenamiento.Ruta != "/storage/emulated/0" {
		t.Errorf("el almacenamiento es %q y se esperaba /storage/emulated/0", j.Almacenamiento.Ruta)
	}
	if len(j.Almacenamiento.Alias) != 1 || j.Almacenamiento.Alias[0] != "/sdcard" {
		t.Errorf("los alias son %v y se esperaba solo /sdcard", j.Almacenamiento.Alias)
	}
}

// TestRomArbolYReglaMasLarga comprueba que el alias se resuelve al único almacenamiento, que el
// árbol sembrado responde a listFiles y que la regla de prefijo más larga vence a la general.
func TestRomArbolYReglaMasLarga(t *testing.T) {
	const escribir = "android.permission.WRITE_EXTERNAL_STORAGE"
	rom, err := cargaRom("", []string{escribir}, "")
	if err != nil {
		t.Fatalf("cargar la ROM: %v", err)
	}
	if rom.guardar != "" {
		t.Errorf("guardar es %q y la persistencia debe estar desactivada por defecto", rom.guardar)
	}
	if canonica := rom.canonica("/sdcard/Download/leeme.txt"); canonica != "/storage/emulated/0/Download/leeme.txt" {
		t.Errorf("el alias dio %q", canonica)
	}
	nodo := rom.arbol["/storage/emulated/0/Download"]
	if nodo == nil || !nodo.directorio {
		t.Fatalf("Download no está sembrado como directorio: %+v", nodo)
	}
	if len(nodo.hijos) != 2 || nodo.hijos[0] != "/storage/emulated/0/Download/leeme.txt" {
		t.Errorf("los hijos de Download son %v y se esperaban dos en orden de siembra", nodo.hijos)
	}
	// sin permiso concedido todo se deniega, incluso bajo el prefijo general
	sinPermiso, err := cargaRom("", nil, "")
	if err != nil {
		t.Fatalf("cargar la ROM sin permisos: %v", err)
	}
	if sinPermiso.accesoPermitido("/sdcard/Download", escribir) {
		t.Error("la ROM concedió acceso sin permiso concedido")
	}
	if !rom.accesoPermitido("/sdcard/Download/leeme.txt", escribir) {
		t.Error("WRITE concedido no da acceso a Download")
	}
	if rom.accesoPermitido("/sdcard/Documents/notas.txt", escribir) {
		t.Error("la regla general concedió acceso a Documents, donde la regla más larga deniega")
	}
}

// maquinaListConFiltro arma una máquina con la ROM sembrada y un filtro del dex cuyo `accept` es el
// cuerpo dado. El dex de la prueba lleva los cuatro métodos de framework que el cuerpo de extensión
// invoca, para que la resolución sea la misma que en la APK del gestor de archivos.
func maquinaListConFiltro(t *testing.T, clase string, insns []uint16, registros int) (*maquinaDalvik, *objetoDalvik) {
	t.Helper()
	rom, err := cargaRom("", []string{"android.permission.READ_EXTERNAL_STORAGE"}, "")
	if err != nil {
		t.Fatalf("cargar la ROM con permiso de lectura: %v", err)
	}
	d := &dexFile{
		strings: []string{".txt"},
		methods: []dexMethod{
			{owner: "Ljava/io/File;", proto: dexProto{returnType: "Ljava/lang/String;"}, name: "getPath"},
			{owner: "Ljava/lang/String;", proto: dexProto{returnType: "I"}, name: "length"},
			{owner: "Ljava/lang/String;", proto: dexProto{params: []string{"I"}, returnType: "Ljava/lang/String;"}, name: "substring"},
			{owner: "Ljava/lang/String;", proto: dexProto{params: []string{"Ljava/lang/Object;"}, returnType: "Z"}, name: "equals"},
		},
	}
	met := &metodoDalvik{nombre: "accept", proto: "(Ljava/io/File;)Z", retorno: "Z", dex: d,
		item: &codeItem{registersSize: uint32(registros), insSize: 2,
			insnsSize: uint32(len(insns)), insns: insns}}
	p := &programaDalvik{dex: d, clases: map[string]*claseDalvik{
		clase: {nombre: clase, super: "Ljava/lang/Object;", metodos: map[string]*metodoDalvik{
			"accept(Ljava/io/File;)Z": met,
		}},
	}}
	m := nuevaMaquina(p, nil)
	m.rom = rom
	return m, &objetoDalvik{clase: clase, campos: map[string]any{}}
}

// nombresListados convierte el arreglo de listFiles o de list a nombres de fichero, sin añadir nada.
func nombresListados(m *maquinaDalvik, v any, forma string) []string {
	a, ok := v.(*arregloDalvik)
	if !ok {
		return nil
	}
	salida := make([]string, 0, len(a.datos))
	for _, e := range a.datos {
		if forma == "listFiles" {
			f, ok := e.(*objetoDalvik)
			if !ok {
				continue
			}
			r := m.rutaDeFichero(f)
			salida = append(salida, r[strings.LastIndex(r, "/")+1:])
		} else {
			salida = append(salida, textoDe(e))
		}
	}
	return salida
}

// TestListFilesConFiltroQueAceptaTodo comprueba el eslabón que faltaba: listFiles con un FileFilter
// de una clase del dex devuelve los hijos sembrados, no un arreglo vacío. Los cuatro ficheros están
// en cuatro directorios reales de la ROM; se recorren todos con el mismo filtro, sin inventar rutas.
func TestListFilesConFiltroQueAceptaTodo(t *testing.T) {
	// con registersSize 3 e insSize 2 el receptor entra en v1 y el File en v2: el cuerpo usa v0.
	m, filtro := maquinaListConFiltro(t, "Lfiltro/Todo;", []uint16{0x1012, 0x000f}, 3)
	quiero := map[string]bool{
		"leeme.txt": false, "gato.png": false, "notas.txt": false, "cancion.mp3": false,
	}
	for _, dir := range []string{"/sdcard/Download", "/sdcard/Download/fotos",
		"/sdcard/Documents", "/sdcard/Music"} {
		v, ok, err := m.intentaStub([]string{"Ljava/io/File;"}, "listFiles",
			"(Ljava/io/FileFilter;)[Ljava/io/File;", []any{filtro}, objetoDeRuta(dir))
		if err != nil || !ok {
			t.Fatalf("listFiles con filtro sobre %s: ok=%v err=%v", dir, ok, err)
		}
		if a, esArreglo := v.(*arregloDalvik); !esArreglo || a.tipo != "[Ljava/io/File;" {
			t.Fatalf("listFiles sobre %s devolvió %T y se esperaba [Ljava/io/File;", dir, v)
		}
		for _, nombre := range nombresListados(m, v, "listFiles") {
			if _, esperado := quiero[nombre]; esperado {
				quiero[nombre] = true
			}
		}
	}
	for nombre, visto := range quiero {
		if !visto {
			t.Errorf("el filtro que acepta todo no devolvió %s", nombre)
		}
	}
	// sin filtro el comportamiento no cambia: el directorio se entrega tal cual, con su carpeta.
	v, ok, err := m.intentaStub([]string{"Ljava/io/File;"}, "list",
		"()[Ljava/lang/String;", nil, objetoDeRuta("/sdcard/Download"))
	if err != nil || !ok {
		t.Fatalf("list sin filtro: ok=%v err=%v", ok, err)
	}
	if nombres := nombresListados(m, v, "list"); len(nombres) != 2 ||
		nombres[0] != "leeme.txt" || nombres[1] != "fotos" {
		t.Errorf("list sin filtro dio %v y se esperaban [leeme.txt fotos]", nombres)
	}
}

// TestListFilesFiltraPorExtension ejecuta el accept de una clase del dex que solo pasa los hijos
// cuyo nombre termina en .txt: getPath + length + substring + equals, todos ellos stubs del modelo.
func TestListFilesFiltraPorExtension(t *testing.T) {
	// registersSize 8 e insSize 2: el filtro entra en v6 y el File en v7. Los temporales van de v0.
	cuerpo := []uint16{
		0x106e, 0x0000, 0x0007, // invoke-virtual {v7}, File.getPath:()Ljava/lang/String;
		0x000c,                 // move-result-object v0
		0x106e, 0x0001, 0x0000, // invoke-virtual {v0}, String.length:()I
		0x010a,                 // move-result v1
		0x4212,                 // const/4 v2, #4
		0x0191, 0x0201,         // sub-int v1, v1, v2
		0x206e, 0x0002, 0x0010, // invoke-virtual {v0, v1}, String.substring:(I)Ljava/lang/String;
		0x030c,                 // move-result-object v3
		0x041a, 0x0000,         // const-string v4, ".txt"
		0x206e, 0x0003, 0x0043, // invoke-virtual {v3, v4}, String.equals:(Ljava/lang/Object;)Z
		0x050a,                 // move-result v5
		0x050f,                 // return v5
	}
	m, filtro := maquinaListConFiltro(t, "Lfiltro/Extension;", cuerpo, 8)
	casos := []struct {
		dir      string
		esperado []string
	}{
		{"/sdcard/Download", []string{"leeme.txt"}},
		{"/sdcard/Download/fotos", []string{}},
		{"/sdcard/Documents", []string{"notas.txt"}},
		{"/sdcard/Music", []string{}},
	}
	for _, caso := range casos {
		v, ok, err := m.intentaStub([]string{"Ljava/io/File;"}, "listFiles",
			"(Ljava/io/FileFilter;)[Ljava/io/File;", []any{filtro}, objetoDeRuta(caso.dir))
		if err != nil || !ok {
			t.Fatalf("listFiles filtrado sobre %s: ok=%v err=%v", caso.dir, ok, err)
		}
		obtenido := nombresListados(m, v, "listFiles")
		if len(obtenido) != len(caso.esperado) {
			t.Errorf("listFiles sobre %s dio %v y se esperaba %v", caso.dir, obtenido, caso.esperado)
			continue
		}
		for i := range obtenido {
			if obtenido[i] != caso.esperado[i] {
				t.Errorf("listFiles sobre %s dio %v y se esperaba %v", caso.dir, obtenido, caso.esperado)
				break
			}
		}
	}
}

// TestListFilesMimeTypeMapNoPierdeLaFilaDelFichero fija el eslabón que la rom del gestor dejó al
// descubierto: la app consulta el tipo MIME del fichero antes de crear su fila. Sin el singleton de
// MimeTypeMap, el receptor nulo tira la fila de leeme.txt y solo aparece la carpeta fotos.
func TestListFilesMimeTypeMapNoPierdeLaFilaDelFichero(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{}, nil)
	singleton, ok, err := m.intentaStub([]string{"Landroid/webkit/MimeTypeMap;"},
		"getSingleton", "()Landroid/webkit/MimeTypeMap;", nil, nil)
	if err != nil || !ok {
		t.Fatalf("getSingleton: ok=%v err=%v", ok, err)
	}
	mapa, esMapa := singleton.(*objetoDalvik)
	if !esMapa || mapa.clase != "Landroid/webkit/MimeTypeMap;" {
		t.Fatalf("getSingleton devolvió %T, se esperaba un MimeTypeMap", singleton)
	}
	for extension, esperado := range map[string]string{"txt": "text/plain", "png": "image/png"} {
		v, ok, err := m.intentaStub([]string{"Landroid/webkit/MimeTypeMap;"},
			"getMimeTypeFromExtension", "(Ljava/lang/String;)Ljava/lang/String;",
			[]any{cadenaObjeto(extension)}, mapa)
		if err != nil || !ok {
			t.Fatalf("getMimeTypeFromExtension(%s): ok=%v err=%v", extension, ok, err)
		}
		if obtenido := textoDe(v); obtenido != esperado {
			t.Errorf("getMimeTypeFromExtension(%s) dio %q y se esperaba %q", extension, obtenido, esperado)
		}
	}
	// una extensión desconocida devuelve null, no un texto inventado: la app aplica su comodín.
	v, ok, err := m.intentaStub([]string{"Landroid/webkit/MimeTypeMap;"},
		"getMimeTypeFromExtension", "(Ljava/lang/String;)Ljava/lang/String;",
		[]any{cadenaObjeto("eeme.txt")}, mapa)
	if err != nil || !ok {
		t.Fatalf("getMimeTypeFromExtension(desconocida): ok=%v err=%v", ok, err)
	}
	if v != nil {
		t.Errorf("una extensión desconocida dio %v y se esperaba null", v)
	}
}

// TestListFilesSplitDelMimeSinPerderLaFila fija el segundo eslabón del render: el gestor parte el
// MIME por «/» para decidir el icono. Sin split, el arreglo nulo hacía fracasar el aget-object y la
// fila del fichero regular desaparecía aunque listFiles lo hubiera entregado.
func TestListFilesSplitDelMimeSinPerderLaFila(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{}, nil)
	casos := []struct {
		cadena string
		regex  string
		partes []string
	}{
		{"text/plain", "/", []string{"text", "plain"}},
		{"application/octet-stream", "/", []string{"application", "octet-stream"}},
		{"text/plain/", "/", []string{"text", "plain"}},
	}
	for _, caso := range casos {
		v, ok, err := m.intentaStub([]string{"Ljava/lang/String;"}, "split",
			"(Ljava/lang/String;)[Ljava/lang/String;",
			[]any{cadenaObjeto(caso.regex)}, cadenaObjeto(caso.cadena))
		if err != nil || !ok {
			t.Fatalf("split(%q, %q): ok=%v err=%v", caso.cadena, caso.regex, ok, err)
		}
		arreglo, esArreglo := v.(*arregloDalvik)
		if !esArreglo || arreglo.tipo != "[Ljava/lang/String;" {
			t.Fatalf("split devolvió %T y se esperaba [Ljava/lang/String;", v)
		}
		if len(arreglo.datos) != len(caso.partes) {
			t.Errorf("split(%q) dio %d partes y se esperaban %d", caso.cadena, len(arreglo.datos), len(caso.partes))
			continue
		}
		for i, parte := range caso.partes {
			if obtenido := textoDe(arreglo.datos[i]); obtenido != parte {
				t.Errorf("split(%q)[%d] dio %q y se esperaba %q", caso.cadena, i, obtenido, parte)
			}
		}
	}
}

// TestLastIndexOfCuentaLaUltimaUnidad cierra el eslabón que el gestor dejó abierto en §376: sin
// `lastIndexOf(I)` el cero del stub hacía que la extensión de leeme.txt se leyera como «eeme.txt».
// La cadena llega como objeto del intérprete (`cadenaObjeto`), así que el índice se cuenta sobre su
// contenido real y no sobre un valor supuesto.
func TestLastIndexOfCuentaLaUltimaUnidad(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{}, nil)
	casos := []struct {
		cadena   string
		codigo   int32
		esperado int32
	}{
		{"leeme.txt", int32('.'), 5},
		{"a.b.c", int32('.'), 3},
		{"abc", int32('z'), -1},
		{"abc", int32('c'), 2},
		{"", int32('a'), -1},
		{"\U0001f600", 0x1f600, 0},
		{"\U0001f600", 0xde00, 1},
		{"\U0001f600", -1, -1},
	}
	for _, caso := range casos {
		v, ok, err := m.intentaStub([]string{"Ljava/lang/String;"}, "lastIndexOf", "(I)I",
			[]any{caso.codigo}, cadenaObjeto(caso.cadena))
		if err != nil || !ok {
			t.Fatalf("lastIndexOf(%d) sobre %q: ok=%v err=%v", caso.codigo, caso.cadena, ok, err)
		}
		obtenido, esEntero := v.(int32)
		if !esEntero || obtenido != caso.esperado {
			t.Errorf("lastIndexOf(%d) sobre %q dio %v (%T) y se esperaba %d",
				caso.codigo, caso.cadena, v, v, caso.esperado)
		}
	}
	// la cadena del gestor entrega «txt» al consultar el MIME: el punto está en la unidad 5,
	// no en la 0, y el trozo siguiente alimenta `getMimeTypeFromExtension`.
	extension, ok, err := m.intentaStub([]string{"Ljava/lang/String;"}, "lastIndexOf", "(I)I",
		[]any{int32('.')}, cadenaObjeto("leeme.txt"))
	if err != nil || !ok || extension != int32(5) {
		t.Fatalf("la extensión de leeme.txt debe empezar en 5; dio %v, ok=%v, err=%v", extension, ok, err)
	}
	punto := extension.(int32)
	nombre, ok, err := m.intentaStub([]string{"Ljava/lang/String;"}, "substring", "(I)Ljava/lang/String;",
		[]any{punto + 1}, cadenaObjeto("leeme.txt"))
	if err != nil || !ok {
		t.Fatalf("substring(%d): ok=%v err=%v", punto+1, ok, err)
	}
	if obtenido := textoDe(nombre); obtenido != "txt" {
		t.Fatalf("la extensión leída debe ser txt; dio %q", obtenido)
	}
	mime, ok, err := m.intentaStub([]string{"Landroid/webkit/MimeTypeMap;"}, "getMimeTypeFromExtension",
		"(Ljava/lang/String;)Ljava/lang/String;", []any{nombre},
		&objetoDalvik{clase: "Landroid/webkit/MimeTypeMap;", campos: map[string]any{}})
	if err != nil || !ok {
		t.Fatalf("getMimeTypeFromExtension(txt): ok=%v err=%v", ok, err)
	}
	if obtenido := textoDe(mime); obtenido != "text/plain" {
		t.Errorf("la extensión txt debe dar text/plain; dio %q", obtenido)
	}
}

// maquinaContenedorConOyente arma un contenedor con dos botones añadidos por código, sin id, y un
// único OnClickListener registrado sobre el segundo. El cuerpo del onClick escribe una cadena fija en
// la vista que recibe, para poder comprobar qué hijo se llevó el clic sin inventar un modelo.
func maquinaContenedorConOyente(t *testing.T) (*maquinaDalvik, uint32, *objetoDalvik, *objetoDalvik) {
	t.Helper()
	d := &dexFile{
		strings: []string{"FILA-1"},
		methods: []dexMethod{{owner: "Landroid/widget/Button;",
			proto: dexProto{params: []string{"Ljava/lang/CharSequence;"}, returnType: "V"}, name: "setText"}},
	}
	const claseOyente = "Loyente/Fila;"
	// registersSize 3 e insSize 2: el listener entra en v1 y la vista pulsada en v2; v0 guarda la
	// cadena que se escribe en esa vista. El método indexado en 0 es Button.setText.
	met := &metodoDalvik{nombre: "onClick", proto: "(Landroid/view/View;)V", retorno: "V", dex: d,
		item: &codeItem{registersSize: 3, insSize: 2, insnsSize: 6,
			insns: []uint16{0x001a, 0x0000, 0x206e, 0x0000, 0x0002, 0x000e}}}
	p := &programaDalvik{dex: d, clases: map[string]*claseDalvik{
		claseOyente: {nombre: claseOyente, super: "Ljava/lang/Object;",
			metodos: map[string]*metodoDalvik{
				"onClick(Landroid/view/View;)V": met,
			}},
	}}
	m := nuevaMaquina(p, nil)
	const id = uint32(0x7f030001) // id/drive_list del gestor de archivos
	primero := &objetoDalvik{clase: "Landroid/widget/Button;", campos: map[string]any{}}
	segundo := &objetoDalvik{clase: "Landroid/widget/Button;", campos: map[string]any{}}
	contenedor := &objetoDalvik{clase: "Landroid/widget/LinearLayout;", id: id,
		campos: map[string]any{}}
	contenedor.hijosCodigo = []*objetoDalvik{primero, segundo}
	m.contenedores = map[uint32][]*objetoDalvik{id: contenedor.hijosCodigo}
	oyente := &objetoDalvik{clase: claseOyente, campos: map[string]any{}}
	m.oyentes = []oyenteRegistrado{{oyente: oyente, vista: segundo}}
	return m, id, primero, segundo
}

// TestPulsarHijoContenedorSinId comprueba la frontera que faltaba para el gestor de archivos: el id
// del contenedor no tiene OnItemClickListener, así que `id/contenedor:N` tiene que resolver el clic
// contra los hijos que la app añadió con addView. El hijo sin listener no se puede pulsar, el que lo
// tiene recibe el onClick y la posición fuera de rango sigue siendo un error.
func TestPulsarHijoContenedorSinId(t *testing.T) {
	m, id, primero, segundo := maquinaContenedorConOyente(t)
	// sin lista con adaptador ni OnItemClickListener, la ruta antigua no puede entregar el clic
	if m.pulsaFila(id, 1) {
		t.Fatal("el contenedor sin OnItemClickListener aceptó un clic de fila")
	}
	if m.pulsaHijoContenedor(id, 0) {
		t.Fatal("se pulsó el hijo 0, que no registró OnClickListener")
	}
	if m.interaccion != 0 || primero.texto != "" {
		t.Errorf("el hijo sin oyente cambió el estado: interacciones=%d texto=%q", m.interaccion, primero.texto)
	}
	if !m.pulsaHijoContenedor(id, 1) {
		t.Fatal("no se pudo pulsar el hijo 1, que sí registró OnClickListener")
	}
	if m.interaccion != 1 {
		t.Errorf("el clic no contó una interacción: %d", m.interaccion)
	}
	if segundo.texto != "FILA-1" {
		t.Errorf("el onClick escribió %q en el hijo 1 y se esperaba «FILA-1»", segundo.texto)
	}
	if avisos := strings.Join(m.avisos, "\n"); avisos != "" {
		t.Errorf("el clic dejó avisos: %s", avisos)
	}
	if m.pulsaHijoContenedor(id, 2) {
		t.Error("se pulsó una posición que no existe en el contenedor")
	}
	if m.pulsaHijoContenedor(id, -1) {
		t.Error("se pulsó una posición negativa del contenedor")
	}
}

// TestRomVolcadoEsReleible cubre la persistencia opcional sin escribir en disco: el volcado tiene
// que volver a cargarse con el mismo almacenamiento y los mismos nodos.
func TestRomVolcadoEsReleible(t *testing.T) {
	rom, err := cargaRom("", nil, "")
	if err != nil {
		t.Fatalf("cargar la ROM: %v", err)
	}
	rom.pone("/sdcard/Download/nuevo.txt", false, "sembrado por la app")
	datos := rom.volcado()
	var j romJSON
	if err := json.Unmarshal(datos, &j); err != nil {
		t.Fatalf("el volcado no es JSON: %v", err)
	}
	if j.Almacenamiento.Ruta != "/storage/emulated/0" {
		t.Errorf("el volcado cambió el almacenamiento a %q", j.Almacenamiento.Ruta)
	}
	encontrado := false
	for _, e := range j.Arbol {
		if e.Ruta == "/storage/emulated/0/Download/nuevo.txt" && e.Contenido == "sembrado por la app" {
			encontrado = true
		}
	}
	if !encontrado {
		t.Errorf("el volcado perdió el fichero creado: %d entradas", len(j.Arbol))
	}
	if _, err := cargaRom(rutaDelFicheroRom(), nil, ""); err != nil {
		t.Errorf("el fichero versionable no se puede cargar: %v", err)
	}
}

// maquinaFutureTaskCallable arma el encadenamiento que el gestor de archivos construye en
// MainActivity.k: un Callable del dex dentro de un FutureTask, metido a su vez en otro FutureTask
// como Runnable. El cuerpo del Callable llama a File.listFiles y guarda el arreglo en un campo de
// la propia instancia, para que la ejecución diferida deje una huella medible sin depender de la app.
func maquinaFutureTaskCallable(t *testing.T) (*maquinaDalvik, *objetoDalvik) {
	t.Helper()
	rom, err := cargaRom("", []string{"android.permission.READ_EXTERNAL_STORAGE"}, "")
	if err != nil {
		t.Fatalf("cargar la ROM con permiso de lectura: %v", err)
	}
	d := &dexFile{
		fields: []dexField{
			{owner: "Lcall/Lista;", typeOf: "Ljava/io/File;", name: "f"},
			{owner: "Lcall/Lista;", typeOf: "Ljava/lang/Object;", name: "resultado"},
		},
		methods: []dexMethod{
			{owner: "Ljava/io/File;", proto: dexProto{returnType: "[Ljava/io/File;"}, name: "listFiles"},
		},
	}
	// registersSize 2 e insSize 1: el Callable entra en v1 (this) y v0 guarda el File y el arreglo.
	cuerpo := []uint16{
		0x1054, 0x0000, // iget-object v0, v1, Lcall/Lista;.f:Ljava/io/File;
		0x106e, 0x0000, 0x0000, // invoke-virtual {v0}, File.listFiles:()[Ljava/io/File;
		0x000c,         // move-result-object v0
		0x105b, 0x0001, // iput-object v0, v1, Lcall/Lista;.resultado:Ljava/lang/Object;
		0x0011,         // return-object v0
	}
	met := &metodoDalvik{nombre: "call", proto: "()Ljava/lang/Object;", retorno: "Ljava/lang/Object;",
		dex: d, item: &codeItem{registersSize: 2, insSize: 1,
			insnsSize: uint32(len(cuerpo)), insns: cuerpo}}
	p := &programaDalvik{dex: d, clases: map[string]*claseDalvik{
		"Lcall/Lista;": {nombre: "Lcall/Lista;", super: "Ljava/lang/Object;",
			metodos: map[string]*metodoDalvik{"call()Ljava/lang/Object;": met}},
	}}
	m := nuevaMaquina(p, nil)
	m.rom = rom
	callable := &objetoDalvik{clase: "Lcall/Lista;", campos: map[string]any{}}
	return m, callable
}

// cadenaFutureTaskCallable reproduce los dos `<init>` de MainActivity.k: un FutureTask(Callable) y
// después un FutureTask(Runnable, Object) que envuelve al primero, que es el que el ejecutor encola.
func cadenaFutureTaskCallable(t *testing.T, m *maquinaDalvik, callable *objetoDalvik) *objetoDalvik {
	t.Helper()
	primero := &objetoDalvik{clase: "Ljava/util/concurrent/FutureTask;", campos: map[string]any{}}
	if _, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/FutureTask;"}, "<init>",
		"(Ljava/util/concurrent/Callable;)V", []any{callable}, primero); err != nil || !ok {
		t.Fatalf("FutureTask(Callable): ok=%v err=%v", ok, err)
	}
	segundo := &objetoDalvik{clase: "Ljava/util/concurrent/FutureTask;", campos: map[string]any{}}
	if _, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/FutureTask;"}, "<init>",
		"(Ljava/lang/Runnable;Ljava/lang/Object;)V", []any{primero, nil}, segundo); err != nil || !ok {
		t.Fatalf("FutureTask(Runnable, Object): ok=%v err=%v", ok, err)
	}
	return segundo
}

// TestFutureTaskEjecutaCallDiferido falsifica la frontera del gestor: hasta ahora el intérprete
// buscaba `run()V` en el FutureTask y no encontraba nada, así que el `Callable.call` nunca corría
// y `id/list` salía vacío. Aquí el cuerpo del Callable lista los cuatro directorios de la ROM.
func TestFutureTaskEjecutaCallDiferido(t *testing.T) {
	m, callable := maquinaFutureTaskCallable(t)
	quiero := map[string]bool{
		"leeme.txt": false, "gato.png": false, "notas.txt": false, "cancion.mp3": false,
	}
	for _, dir := range []string{"/sdcard/Download", "/sdcard/Download/fotos",
		"/sdcard/Documents", "/sdcard/Music"} {
		callable.campos["Lcall/Lista;.f"] = objetoDeRuta(dir)
		segundo := cadenaFutureTaskCallable(t, m, callable)
		if err := m.ejecutaRunnable(segundo, "ExecutorService.execute"); err != nil {
			t.Fatalf("ejecutar el FutureTask sobre %s: %v", dir, err)
		}
		v, hay := callable.campos["Lcall/Lista;.resultado"]
		if !hay {
			t.Fatalf("el Callable.call no corrió sobre %s: sin resultado; avisos %v", dir, m.avisos)
		}
		for _, nombre := range nombresListados(m, v, "listFiles") {
			if _, esperado := quiero[nombre]; esperado {
				quiero[nombre] = true
			}
		}
	}
	for nombre, visto := range quiero {
		if !visto {
			t.Errorf("el Callable.call diferido no listó %s; avisos %v", nombre, m.avisos)
		}
	}
}

// TestFutureTaskNoRompeRunnableRun guarda el camino de §361: un Runnable del dex que no es un
// FutureTask sigue ejecutando su `run()V`, sin que la envoltura nueva lo desvíe hacia `call()`.
func TestFutureTaskNoRompeRunnableRun(t *testing.T) {
	rom, err := cargaRom("", []string{"android.permission.READ_EXTERNAL_STORAGE"}, "")
	if err != nil {
		t.Fatalf("cargar la ROM con permiso de lectura: %v", err)
	}
	d := &dexFile{
		fields: []dexField{
			{owner: "Lrunnable/Plano;", typeOf: "Ljava/io/File;", name: "f"},
			{owner: "Lrunnable/Plano;", typeOf: "Ljava/lang/Object;", name: "resultado"},
		},
		methods: []dexMethod{
			{owner: "Ljava/io/File;", proto: dexProto{returnType: "[Ljava/io/File;"}, name: "listFiles"},
		},
	}
	cuerpo := []uint16{
		0x1054, 0x0000, // iget-object v0, v1, Lrunnable/Plano;.f:Ljava/io/File;
		0x106e, 0x0000, 0x0000, // invoke-virtual {v0}, File.listFiles:()[Ljava/io/File;
		0x000c,         // move-result-object v0
		0x105b, 0x0001, // iput-object v0, v1, Lrunnable/Plano;.resultado:Ljava/lang/Object;
		0x000e,         // return-void
	}
	met := &metodoDalvik{nombre: "run", proto: "()V", retorno: "V", dex: d,
		item: &codeItem{registersSize: 2, insSize: 1,
			insnsSize: uint32(len(cuerpo)), insns: cuerpo}}
	p := &programaDalvik{dex: d, clases: map[string]*claseDalvik{
		"Lrunnable/Plano;": {nombre: "Lrunnable/Plano;", super: "Ljava/lang/Object;",
			metodos: map[string]*metodoDalvik{"run()V": met}},
	}}
	m := nuevaMaquina(p, nil)
	m.rom = rom
	runnable := &objetoDalvik{clase: "Lrunnable/Plano;", campos: map[string]any{
		"Lrunnable/Plano;.f": objetoDeRuta("/sdcard/Download"),
	}}
	if err := m.ejecutaRunnable(runnable, "ExecutorService.execute"); err != nil {
		t.Fatalf("ejecutar el Runnable corriente: %v", err)
	}
	if _, hay := runnable.campos["Lrunnable/Plano;.resultado"]; !hay {
		t.Errorf("el Runnable corriente no ejecutó su run()V; avisos %v", m.avisos)
	}
}

// tablaDeDrawablesDelGestor escribe los siete drawables medidos en el APK del gestor con el tipo 2,
// de modo que sus ids son 0x7f020000 a 0x7f020006, como los que trae el resources.arsc de verdad.
func tablaDeDrawablesDelGestor() tablaEscribir {
	claves := []string{"archive", "audio", "file", "folder", "picture", "unknown", "video"}
	entradas := make([]entradaEscribir, 0, len(claves))
	for i := range claves {
		entradas = append(entradas, entradaEscribir{clave: i,
			valor: valorEscribir{tipo: 0x03, datos: uint32(i)}})
	}
	return tablaEscribir{
		idPaquete: 0x7f, nombrePaquete: "com.prueba.gestor",
		nombresTipo: []string{"id", "drawable"}, nombresUTF8: false,
		claves: claves, clavesUTF8: true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{{clave: 0,
					valor: valorEscribir{tipo: 0x03, datos: 0}}}}}},
			{id: 2, specFlags: make([]uint32, len(claves)),
				bloques: []bloqueEscribir{{entradas: entradas}}},
		},
	}
}

func TestRecursoDeFilaDelGestorSaleDelArsc(t *testing.T) {
	arsc, err := escribeTablaRecursos(tablaDeDrawablesDelGestor())
	if err != nil {
		t.Fatalf("la tabla de drawables no se escribe: %v", err)
	}
	tabla, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla de drawables no se relee: %v", err)
	}
	casos := []struct{ nombre, quiero string }{
		{"fotos", "0x7f020003"},
		{"leeme.txt", "0x7f020002"},
		{"gato.png", "0x7f020004"},
		{"cancion.mp3", "0x7f020001"},
		{"pelicula.mp4", "0x7f020006"},
		{"paquete.zip", "0x7f020000"},
		{"misterio.xyz", "0x7f020005"},
	}
	for _, c := range casos {
		n := nodoDeFila(filaPintada{Tipo: "c.b", Texto: c.nombre}, tabla, nil)
		if n.Recurso != c.quiero {
			t.Errorf("%s publica recurso=%q y se esperaba %s", c.nombre, n.Recurso, c.quiero)
		}
	}
	// los rótulos del encabezado y los textos de estado son literales del APK, no ficheros
	for _, rotulo := range []string{" <- ", " ||| ", "Loading...", "cutting/copying/deleting files..."} {
		if clave := drawableDeExtension(rotulo); clave != "" {
			t.Errorf("el rótulo %q se convirtió en el drawable %q", rotulo, clave)
		}
	}
}

func TestAnotaVisibilidadDeLaEjecucion(t *testing.T) {
	const id = uint32(0x7f030008) // id/menu_list del gestor
	m := nuevaMaquina(&programaDalvik{}, nil)
	m.vistas[id] = &objetoDalvik{clase: "Landroid/widget/LinearLayout;", id: id,
		campos: map[string]any{"vista.visibilidad": int32(vistaGone)}}
	raiz := &nodoVista{IDRef: "0x7f030008", Hijos: []*nodoVista{{IDRef: "0x7fffffff"}}}
	anotaVisibilidad(raiz, &resultadoEjecucion{maquina: m})
	if raiz.Visibilidad != "gone" || raiz.visibilidad != vistaGone {
		t.Errorf("el nodo dice visibilidad=%q (%d) y la app pidió gone", raiz.Visibilidad, raiz.visibilidad)
	}
	if hijo := raiz.Hijos[0]; hijo.Visibilidad != "" || hijo.visibilidad != vistaVisible {
		t.Errorf("un hijo sin llamada cambió a visibilidad=%q (%d)", hijo.Visibilidad, hijo.visibilidad)
	}
	// sin ejecución no se oculta nada: la visibilidad del layout se conserva
	otro := &nodoVista{IDRef: "0x7f030008"}
	anotaVisibilidad(otro, nil)
	if otro.Visibilidad != "" {
		t.Errorf("sin ejecución se escribió visibilidad=%q", otro.Visibilidad)
	}
}

func TestTextoDeFilaCodigoUsaElUltimoSetText(t *testing.T) {
	fila := &objetoDalvik{clase: "Landroid/widget/Button;", campos: map[string]any{},
		texto: "", textoEscrito: true}
	fila.hijosCodigo = []*objetoDalvik{{texto: "Loading...", textoEscrito: true}}
	if texto := textoDeFilaCodigo(fila); texto != "" {
		t.Errorf("la fila dice %q; el último setText de la vista fue vacío", texto)
	}
}

// TestVisibilidadMedidaDeVistaFalsaLosTransitorios delimita la señal: la retirada y el setVisibility
// son estado medido; el texto de la vista no se mira nunca, porque un fichero legítimo podría
// llamarse «Loading...». El id sintético 0x7ffffffe es el que el dex pone a la vista creada en código.
func TestVisibilidadMedidaDeVistaFalsaLosTransitorios(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{}, nil)
	transitoria := &objetoDalvik{clase: "Landroid/widget/TextView;", id: 0x7ffffffe,
		campos: map[string]any{}, texto: "Loading..."}
	m.vistasRetiradas = map[uint32]bool{0x7ffffffe: true}
	if visibilidadMedidaDeVista(transitoria, m) {
		t.Error("la vista retirada sigue visible para el modelo")
	}
	// otra vista con el MISMO texto, sin marca de retirada, se publica: no hay filtro de cadenas
	legitima := &objetoDalvik{clase: "Landroid/widget/TextView;", id: 0,
		campos: map[string]any{}, texto: "Loading..."}
	if !visibilidadMedidaDeVista(legitima, m) {
		t.Error("se descartó una vista legítima por su texto")
	}
	oculta := &objetoDalvik{clase: "Landroid/widget/TextView;", id: 0,
		campos: map[string]any{"vista.visibilidad": int32(vistaGone)}}
	if visibilidadMedidaDeVista(oculta, m) {
		t.Error("una vista puesta gone sigue visible para el modelo")
	}
	if !visibilidadMedidaDeVista(&objetoDalvik{campos: map[string]any{}}, m) {
		t.Error("una vista sin marcas se descartó sin motivo medido")
	}
}

// TestCampoVisibleEnElJSONDelNodo comprueba que la visibilidad medida viaja en cada nodo publicado
// y que la ausencia de llamada no se inventa un `gone`.
func TestCampoVisibleEnElJSONDelNodo(t *testing.T) {
	n := &nodoVista{Tipo: "LinearLayout", Visible: true,
		Hijos: []*nodoVista{{Tipo: "TextView", Visible: false, Visibilidad: "gone"}}}
	datos, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("el nodo no se serializa: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(datos, &doc); err != nil {
		t.Fatalf("el JSON del nodo no se relee: %v", err)
	}
	if visible, ok := doc["visible"].(bool); !ok || !visible {
		t.Errorf("el nodo visible dice visible=%v", doc["visible"])
	}
	hijo := doc["hijos"].([]any)[0].(map[string]any)
	if visible, ok := hijo["visible"].(bool); !ok || visible {
		t.Errorf("el nodo gone dice visible=%v", hijo["visible"])
	}
}

// TestRegistraRetiradasSobreviveAlRefresh fija la propiedad que el gestor necesita: el id sintético
// retirado queda en la máquina aunque la vista se vuelva a crear con un objeto nuevo.
func TestRegistraRetiradasSobreviveAlRefresh(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{}, nil)
	retirada := &objetoDalvik{clase: "Landroid/widget/TextView;", id: 0x7ffffffe,
		campos: map[string]any{}, texto: "Loading..."}
	contenedor := &objetoDalvik{clase: "Landroid/widget/LinearLayout;", id: 0x7f030005,
		campos: map[string]any{}, hijosCodigo: []*objetoDalvik{retirada}}
	m.registraRetiradas(contenedor)
	nueva := &objetoDalvik{clase: "Landroid/widget/TextView;", id: 0x7ffffffe,
		campos: map[string]any{}, texto: "Loading..."}
	if visibilidadMedidaDeVista(nueva, m) {
		t.Error("la vista recreada tras el refresh volvió a publicarse")
	}
}
