package main

// Las pruebas de la Fase 2 de la vista previa: el intérprete Dalvik.
//
// El dex de las pruebas no se versiona (regla 2): se genera en caliente con el emisor del fork
// (`compilarProyectoJava` + `dexDeClases`), que es el mismo camino que usa `apkc build --compiler go`.
// El R.java también se genera con `rDeTabla` desde la tabla que escribe `arscwriter.go`, para que los
// ids del fuente no discrepen de los del apk.
//
// La prueba que cierra el criterio de aceptación de la fase es
// TestElInterpreteEjecutaOnCreateYCapturaElSetText: la plantilla Contador del studio corre y su
// `total.setText("Pulsos: " + pulsos)` sale capturado.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
)

// dexDeFuentes compila un proyecto Java con el emisor del fork y devuelve su classes.dex.
func dexDeFuentes(t *testing.T, fuentes ...fuenteJava) []byte {
	t.Helper()
	emitidas, err := compilarProyectoJava(fuentes, jarDelSDK(t))
	if err != nil {
		t.Fatalf("las fuentes no compilan: %v", err)
	}
	completas := make([]classFull, 0, len(emitidas))
	for _, c := range emitidas {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", c.interno, err)
		}
		completas = append(completas, cf)
	}
	datos, err := dexDeClases(completas)
	if err != nil {
		t.Fatalf("el dex no sale: %v", err)
	}
	return datos
}

// tablaDelContador es la tabla de recursos de la plantilla Contador: los dos ids del layout y el
// layout, con la ruta en el pool global, que es lo que resuelve un id de `R.layout`.
func tablaDelContador() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.contador",
		global:        []string{"res/layout/main.xml", "Hola desde un recurso"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout", "string"},
		nombresUTF8:   false,
		claves:        []string{"pulsar", "total", "main", "saludo"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 2, valor: valorEscribir{tipo: 0x03, datos: 0}},
				}}},
			},
			{
				id:        3,
				specFlags: []uint32{0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 3, valor: valorEscribir{tipo: 0x03, datos: 1}},
				}}},
			},
		},
	}
}

// layoutDelContador es el layout de la plantilla, con los dos ids que usa su actividad.
func layoutDelContador() documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 2}},
		utf8: true,
		raiz: &elementoEscribir{
			nombre: "LinearLayout",
			linea:  2,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "orientation", id: 0x010100c4, tipo: tipoEnteroDec, datos: 1},
				{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xffffffff},
				{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xffffffff},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "TextView",
					linea:  8,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010001},
						{nombre: "style", tipo: tipoReferencia, datos: 0x01030042},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Pulsos: 0"},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: 0x7f010000},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: "Pulsar"},
					},
				},
			},
		},
	}
}

// manifiestoDelContador: la actividad lanzadora es `.MainActivity`.
func manifiestoDelContador() documentoAXML { return manifiestoConActividad(".MainActivity") }

// manifiestoConActividad arma el mismo manifiesto con otra actividad lanzadora.
func manifiestoConActividad(actividad string) documentoAXML {
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.contador"},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "application",
					linea:  9,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Contador"},
					},
					hijos: []*elementoEscribir{
						{
							nombre: "activity",
							linea:  11,
							attrs: []atributoEscribir{
								{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: actividad},
							},
							hijos: []*elementoEscribir{
								{
									nombre: "intent-filter",
									linea:  1,
									hijos: []*elementoEscribir{
										{
											nombre: "action",
											linea:  2,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
													texto: "android.intent.action.MAIN"},
											},
										},
										{
											nombre: "category",
											linea:  3,
											attrs: []atributoEscribir{
												{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena,
													texto: "android.intent.category.LAUNCHER"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// fuenteContadorStudio es la actividad de la plantilla Contador del studio, copiada de su texto
// generado: campo, listener anónimo y concatenación con un entero.
const fuenteContadorStudio = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText("Pulsos: " + pulsos);
            }
        });
    }
}
`

// fuentesDelContador arma el proyecto: el R.java sale de la tabla, para que los ids no discrepen.
func fuentesDelContador(t *testing.T) []fuenteJava {
	t.Helper()
	return []fuenteJava{
		{nombre: "R.java", contenido: rContador(t)},
		{nombre: "MainActivity.java", contenido: fuenteContadorStudio},
	}
}

// rContador es el R.java de la plantilla Contador, generado desde la tabla del propio fork.
func rContador(t *testing.T) string {
	t.Helper()
	r, err := rDeTabla(tablaDelContador())
	if err != nil {
		t.Fatalf("el R.java no sale de la tabla: %v", err)
	}
	return r
}

// TestElInterpreteEjecutaOnCreateYCapturaElSetText es el criterio de aceptación de la Fase 2 sobre el
// dex, sin apk: el onCreate del Contador corre, el layout sale de su setContentView y NO se pulsa
// nada, así que no hay `setText` que capturar y la pantalla se queda en «Pulsos: 0». El listener
// queda registrado y publicado en `oyentes`, que es a quien se le puede pedir un clic.
func TestElInterpreteEjecutaOnCreateYCapturaElSetText(t *testing.T) {
	d, err := readDex(dexDeFuentes(t, fuentesDelContador(t)...))
	if err != nil {
		t.Fatalf("el dex del Contador no se relee: %v", err)
	}
	// la primera ejecución va sin tabla (el id se queda en su número) y la segunda con la del propio
	// fork, que sí lo nombra
	res, layoutID := ejecutaOnCreate(d, nil, "Lcom/hola/contador/MainActivity;")
	if res.Error != "" {
		t.Fatalf("el onCreate falló: %s", res.Error)
	}
	if res.Actividad != "com.hola.contador.MainActivity" {
		t.Errorf("se ejecutó %q y la lanzadora es com.hola.contador.MainActivity", res.Actividad)
	}
	if res.Metodo != "onCreate" {
		t.Errorf("se ejecutó %q y se esperaba onCreate", res.Metodo)
	}
	if layoutID != 0x7f020000 {
		t.Errorf("el setContentView dio el layout %#08x y R.layout.main es 0x7f020000", layoutID)
	}
	if res.Pasos == 0 {
		t.Error("el intérprete no contó ni un paso")
	}
	if res.Interacciones != 0 {
		t.Errorf("el emulador pulsó %d veces sin que nadie tocara la pantalla", res.Interacciones)
	}
	if len(res.Capturas) != 0 {
		t.Fatalf("sin clic no hay escrituras que capturar: %+v", res.Capturas)
	}
	if len(res.Oyentes) != 1 || res.Oyentes[0].IDRef != "0x7f010000" {
		t.Fatalf("los oyentes son %+v y el Contador registra uno sobre id/pulsar (0x7f010000)", res.Oyentes)
	}
	if len(res.Avisos) != 0 {
		t.Errorf("la ejecución dejó avisos que no debería: %v", res.Avisos)
	}
	// y con la tabla del propio fork el id se nombra
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla del Contador no se escribe: %v", err)
	}
	tablaFork, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla del Contador no se relee: %v", err)
	}
	res, _ = ejecutaOnCreate(d, tablaFork, "Lcom/hola/contador/MainActivity;")
	if len(res.Oyentes) != 1 || res.Oyentes[0].ID != "id/pulsar" {
		t.Fatalf("con la tabla del fork el oyente es %+v y se esperaba id/pulsar", res.Oyentes)
	}
}

// TestElPreviewLlevaElTextoDinamicoAlModelo cierra la fase de punta a punta sin sdk de por medio: un
// apk armado en memoria con los escritores del fork y el classes.dex del Contador. Sin clic, el nodo
// conserva el texto declarado y el botón queda marcado como oyente: es el emulador en reposo.
func TestElPreviewLlevaElTextoDinamicoAlModelo(t *testing.T) {
	// la lanzadora es Casos: es donde vive el switch, y así lo recorre también el comando entero
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelContador())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk del Contador no se deja leer: %v", err)
	}
	if m.Ejecucion == nil {
		t.Fatal("el modelo no trae la ejecución")
	}
	if m.Ejecucion.Error != "" {
		t.Fatalf("la ejecución falló: %s", m.Ejecucion.Error)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("el layout se resolvió como %q", m.Layout)
	}
	if m.Raiz == nil || len(m.Raiz.Hijos) != 2 {
		t.Fatalf("el modelo no tiene los dos hijos del layout: %+v", m.Raiz)
	}
	texto, boton := m.Raiz.Hijos[0], m.Raiz.Hijos[1]
	if texto.Texto != "Pulsos: 0" {
		t.Errorf("el texto declarado del TextView es %q", texto.Texto)
	}
	if texto.TextoDinamico != "" {
		t.Errorf("sin clic el TextView no puede tener texto dinámico: %q", texto.TextoDinamico)
	}
	if texto.Oyente {
		t.Error("el TextView declara un oyente que solo tiene el botón")
	}
	if boton.Texto != "Pulsar" || boton.TextoDinamico != "" {
		t.Errorf("el Button se lee como %q / %q y nadie le escribió texto", boton.Texto, boton.TextoDinamico)
	}
	if !boton.Oyente {
		t.Error("el Button registra el OnClickListener y el modelo no lo marca como oyente")
	}
	// el ancho estimado usa el texto que se ve: «Pulsos: 0» mide lo mismo que «Pulsos: 1»
	if texto.Rect.Ancho != len([]rune("Pulsos: 0"))*dpPorCaracter+dpRellenoTexto {
		t.Errorf("el ancho del TextView es %d", texto.Rect.Ancho)
	}
}

// apkDelContador arma en memoria el apk de la plantilla Contador con las fuentes que se le den: el
// manifiesto, la tabla y el layout son los del studio, y el dex sale del emisor del fork.
func apkDelContador(t *testing.T, fuentes ...fuenteJava) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelContador())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentes...),
	})
}

// TestElPreviewPulsaElListenerQueSeLePide es el criterio del emulador con clics: sin `--pulsar` la
// pantalla queda en «Pulsos: 0»; con un clic en id/pulsar, en «Pulsos: 1»; y con el mismo clic dos
// veces, en «Pulsos: 2», porque los dos se entregan dentro de la misma ejecución. El id se puede
// pedir por nombre o por su `idRef`, y las dos formas dan el mismo JSON.
func TestElPreviewPulsaElListenerQueSeLePide(t *testing.T) {
	apk := apkDelContador(t, fuentesDelContador(t)...)
	textoDinamico := func(m *modeloVista) string {
		if m.Ejecucion == nil || m.Raiz == nil || len(m.Raiz.Hijos) == 0 {
			t.Fatalf("el modelo no trae ejecución o hijos: %+v", m)
		}
		return m.Raiz.Hijos[0].TextoDinamico
	}

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk en reposo no se lee: %v", err)
	}
	if got := textoDinamico(reposo); got != "" {
		t.Errorf("sin clic el texto dinámico es %q", got)
	}
	if reposo.Ejecucion.Interacciones != 0 || len(reposo.Ejecucion.Capturas) != 0 {
		t.Errorf("sin clic no hay interacciones ni capturas: %+v", reposo.Ejecucion)
	}

	uno, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar"})
	if err != nil {
		t.Fatalf("el clic en id/pulsar falló: %v", err)
	}
	if got := textoDinamico(uno); got != "Pulsos: 1" {
		t.Errorf("tras un clic el texto dinámico es %q y se esperaba «Pulsos: 1»", got)
	}
	if uno.Ejecucion.Interacciones != 1 {
		t.Errorf("interacciones = %d tras un clic", uno.Ejecucion.Interacciones)
	}
	if len(uno.Ejecucion.Capturas) != 1 || uno.Ejecucion.Capturas[0].ID != "id/total" ||
		uno.Ejecucion.Capturas[0].Texto != "Pulsos: 1" {
		t.Errorf("la captura del clic es %+v y se esperaba «Pulsos: 1» sobre id/total", uno.Ejecucion.Capturas)
	}
	if len(uno.Ejecucion.Pulsaciones) != 1 || uno.Ejecucion.Pulsaciones[0] != "id/pulsar" {
		t.Errorf("las pulsaciones entregadas son %v", uno.Ejecucion.Pulsaciones)
	}

	// el mismo clic dos veces: el estado que ve el segundo onClick incluye el primero
	dos, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar", "id/pulsar"})
	if err != nil {
		t.Fatalf("los dos clics fallaron: %v", err)
	}
	if got := textoDinamico(dos); got != "Pulsos: 2" {
		t.Errorf("tras dos clics el texto dinámico es %q y se esperaba «Pulsos: 2»", got)
	}
	if dos.Ejecucion.Interacciones != 2 {
		t.Errorf("interacciones = %d tras dos clics", dos.Ejecucion.Interacciones)
	}
	if len(dos.Ejecucion.Capturas) != 2 || dos.Ejecucion.Capturas[1].Texto != "Pulsos: 2" {
		t.Errorf("las capturas de dos clics son %+v", dos.Ejecucion.Capturas)
	}

	// por su idRef: el mismo resultado, que es lo que publica cada nodo del modelo
	porRef, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"0x7f010000"})
	if err != nil {
		t.Fatalf("el clic por idRef falló: %v", err)
	}
	if got := textoDinamico(porRef); got != "Pulsos: 1" {
		t.Errorf("el clic por idRef dejó %q", got)
	}
	// y por el decimal del mismo id (0x7f010000 = 2130771968)
	porDecimal, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"2130771968"})
	if err != nil {
		t.Fatalf("el clic por el decimal del id falló: %v", err)
	}
	if got := textoDinamico(porDecimal); got != "Pulsos: 1" {
		t.Fatalf("el clic por el decimal del id dejó %q", got)
	}
}

// TestElPreviewRechazaUnClicQueNoExiste: pedir un clic que no se puede entregar no devuelve un modelo
// a medias. El error dice qué listeners sí están registrados, que es lo que hace falta para arreglar
// la llamada.
func TestElPreviewRechazaUnClicQueNoExiste(t *testing.T) {
	apk := apkDelContador(t, fuentesDelContador(t)...)
	if _, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/no-existe"}); err == nil {
		t.Fatal("se aceptó un clic sobre una vista que no existe")
	} else if !strings.Contains(err.Error(), "la tabla no tiene") {
		t.Errorf("el error de un id que no está en la tabla es %q", err)
	}
	// una vista con id pero sin listener tampoco responde: el TextView no tiene onClick, y el error
	// dice qué sí se puede pulsar
	if _, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/total"}); err == nil {
		t.Fatal("se aceptó un clic sobre una vista sin OnClickListener")
	} else if !strings.Contains(err.Error(), "ninguna vista con ese id") ||
		!strings.Contains(err.Error(), "id/pulsar") {
		t.Errorf("el error es %q y tenía que nombrar los listeners registrados", err)
	}
	// y una forma que no es un id se dice, en vez de pulsar cualquier cosa
	if _, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"pulsar"}); err == nil {
		t.Fatal("se aceptó «pulsar» como id")
	}
}

// TestLaConsolaSeparaLaSalidaDeLaApp: `Log.d` y `System.out.println` son la salida de la app y van a
// `consola`, numeradas en el orden de emisión; el registro del emulador (`capturas`) las conserva
// junto a la escritura de la view, que es lo que el panel de registro sigue leyendo.
func TestLaConsolaSeparaLaSalidaDeLaApp(t *testing.T) {
	const conConsola = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.util.Log;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText("Pulsos: " + pulsos);
                Log.d("Contador", "pulso " + pulsos);
                System.out.println("consola " + pulsos);
            }
        });
    }
}
`
	apk := apkDelContador(t, fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: conConsola})
	modelo, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar"})
	if err != nil {
		t.Fatalf("el apk con consola no se ejecuta: %v", err)
	}
	e := modelo.Ejecucion
	if e == nil || e.Error != "" {
		t.Fatalf("la ejecución falló: %+v", e)
	}
	if len(e.Consola) != 2 {
		t.Fatalf("la consola trae %d líneas y la app imprime dos: %+v", len(e.Consola), e.Consola)
	}
	if e.Consola[0].Origen != "Log.d" || e.Consola[0].Texto != "pulso 1" || e.Consola[0].Secuencia != 1 {
		t.Errorf("la primera línea de consola es %+v", e.Consola[0])
	}
	if e.Consola[1].Origen != "println" || e.Consola[1].Texto != "consola 1" || e.Consola[1].Secuencia != 2 {
		t.Errorf("la segunda línea de consola es %+v", e.Consola[1])
	}
	// el registro del emulador no pierde nada: setText, el Log y el println, en ese orden
	origenes := []string{}
	for _, c := range e.Capturas {
		origenes = append(origenes, c.Origen)
	}
	if strings.Join(origenes, ",") != "TextView.setText,Log.d,println" {
		t.Errorf("las capturas del registro son %v", origenes)
	}
	if modelo.Raiz.Hijos[0].TextoDinamico != "Pulsos: 1" {
		t.Errorf("el clic no llegó al modelo: %q", modelo.Raiz.Hijos[0].TextoDinamico)
	}
}

// TestElPreviewSinEjecutarNoInterpreta: la bandera de escape deja el modelo de la Fase 1, aunque el
// apk traiga un dex que sí se ejecuta.
func TestElPreviewSinEjecutarNoInterpreta(t *testing.T) {
	manifiesto, _ := escribeAXML(manifiestoDelContador())
	layout, _ := escribeAXML(layoutDelContador())
	arsc, _ := escribeTablaRecursos(tablaDelContador())
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         dexDeFuentes(t, fuentesDelContador(t)...),
	})
	m, err := modeloVistaDeAPKCon(apk, "", false)
	if err != nil {
		t.Fatalf("el apk no se lee: %v", err)
	}
	if m.Ejecucion != nil {
		t.Errorf("se ejecutó el dex con --sin-ejecutar: %+v", m.Ejecucion)
	}
	if m.Raiz.Hijos[0].TextoDinamico != "" {
		t.Errorf("sin ejecutar no puede haber texto dinámico: %q", m.Raiz.Hijos[0].TextoDinamico)
	}
}

// TestElInterpreteDecodificaCuentasRamasArreglosYAnchos cubre el decodificador con una sonda: bucles,
// `if`, `switch`, arreglos, `long`, cadenas y las dos salidas (`println` y `Log.d`).
func TestElInterpreteDecodificaCuentasRamasArreglosYAnchos(t *testing.T) {
	const sonda = `package com.myapp;

import android.util.Log;

public class Sonda {
    static int suma(int n) { int t = 0; for (int i = 0; i < n; i++) { t += i; } return t; }
    static long producto(long a, long b) { return a * b + 1; }
    static int arreglo() { int[] v = new int[3]; v[0] = 1; v[1] = 2; v[2] = 3; return v[0] + v[1] + v[2]; }
    static int ramas(int x) { if (x > 10) { return 1; } else if (x == 10) { return 2; } return 3; }
    static int casos(int x) { switch (x) { case 1: return 10; case 2: return 20; default: return 0; } }
    static int cuenta(int x) { int i = 0; while (i < x) { i = i + 1; } return i; }

    public static void main(String[] args) {
        System.out.println("suma " + suma(5));
        System.out.println(producto(3, 4));
        System.out.println("arreglo " + arreglo());
        System.out.println("ramas " + ramas(10));
        System.out.println("casos " + casos(2));
        System.out.println("cuenta " + cuenta(4));
        Log.d("Sonda", "listo");
    }
}
`
	d, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Sonda.java", contenido: sonda}))
	if err != nil {
		t.Fatalf("la sonda no se relee: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/myapp/Sonda;")
	if res.Error != "" {
		t.Fatalf("la sonda falló: %s", res.Error)
	}
	if res.Metodo != "main" || res.Actividad != "com.myapp.Sonda" {
		t.Errorf("se ejecutó %s.%s", res.Actividad, res.Metodo)
	}
	quieren := []string{"suma 10", "13", "arreglo 6", "ramas 2", "casos 20", "cuenta 4", "listo"}
	obtenidos := make([]string, 0, len(res.Capturas))
	for _, c := range res.Capturas {
		obtenidos = append(obtenidos, c.Texto)
	}
	if strings.Join(obtenidos, " | ") != strings.Join(quieren, " | ") {
		t.Errorf("las capturas son\n  %v\ny se esperaban\n  %v", obtenidos, quieren)
	}
	if res.Capturas[len(res.Capturas)-1].Origen != "Log.d" {
		t.Errorf("la última captura viene de %q y es un Log.d", res.Capturas[len(res.Capturas)-1].Origen)
	}
}

// TestElInterpreteEligeLaActividadYNoLaAnonima: el fallo medido con NekoDroid sobre un apk real era
// coger la clase anónima. Aunque se proponga `MainActivity$1`, la elegida tiene que ser la actividad.
func TestElInterpreteEligeLaActividadYNoLaAnonima(t *testing.T) {
	d, err := readDex(dexDeFuentes(t, fuentesDelContador(t)...))
	if err != nil {
		t.Fatal(err)
	}
	prog, err := cargaPrograma(d)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	if _, ok := prog.clases["Lcom/hola/contador/MainActivity$1;"]; !ok {
		t.Fatal("la anónima no está en el dex: la prueba no estaría midiendo nada")
	}
	for _, preferida := range []string{"", "Lcom/hola/contador/MainActivity$1;",
		"Lcom/hola/contador/NoExiste;"} {
		clase, conMain := eligeClaseLanzadora(prog, preferida)
		if clase != "Lcom/hola/contador/MainActivity;" || conMain {
			t.Errorf("con preferida %q se eligió %q (main=%v)", preferida, clase, conMain)
		}
	}
	// y una clase con main gana a una con solo onCreate
	if clase, conMain := eligeClaseLanzadora(prog, ""); conMain {
		t.Errorf("%q se tomó por una clase con main", clase)
	}
}

// TestElDescriptorDeLaActividadCubreLasTresFormas: `.MainActivity`, `MainActivity` y el nombre
// completo son el mismo type_id.
func TestElDescriptorDeLaActividadCubreLasTresFormas(t *testing.T) {
	for _, nombre := range []string{".MainActivity", "MainActivity", "com.hola.contador.MainActivity"} {
		if got := descriptorDeActividad("com.hola.contador", nombre); got != "Lcom/hola/contador/MainActivity;" {
			t.Errorf("%q dio %q", nombre, got)
		}
	}
	if got := descriptorDeActividad("com.hola.contador", ""); got != "" {
		t.Errorf("sin nombre salió %q", got)
	}
}

// TestElConstructorDeStringBuilderLlevaSuPrimerTrozo: javac compila `"Pulsos: " + pulsos` como
// `new StringBuilder("Pulsos: ")` + `append(int)` + `toString`. Si el `<init>` se limita a no hacer
// nada, el texto sale sin su parte literal (medido sobre el apk del Contador: «1» y no «Pulsos: 1»).
// La forma con un entero es la CAPACIDAD, no contenido: no puede acabar dentro del texto.
func TestElConstructorDeStringBuilderLlevaSuPrimerTrozo(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	for _, caso := range []struct {
		proto string
		args  []any
		quien string
	}{
		{"(Ljava/lang/String;)V", []any{"Pulsos: "}, "Pulsos: 1"},
		{"()V", nil, "1"},
		{"(I)V", []any{int32(16)}, "1"},
	} {
		sb := &objetoDalvik{clase: "Ljava/lang/StringBuilder;", campos: map[string]any{}}
		candidatos := []string{"Ljava/lang/StringBuilder;"}
		if _, ok, err := m.intentaStub(candidatos, "<init>", caso.proto, caso.args, sb); !ok || err != nil {
			t.Fatalf("el constructor %s no se atendió: ok=%v err=%v", caso.proto, ok, err)
		}
		if _, _, err := m.intentaStub(candidatos, "append", "(I)Ljava/lang/StringBuilder;",
			[]any{int32(1)}, sb); err != nil {
			t.Fatalf("append: %v", err)
		}
		v, ok, err := m.intentaStub(candidatos, "toString", "()Ljava/lang/String;", nil, sb)
		if !ok || err != nil {
			t.Fatalf("toString: ok=%v err=%v", ok, err)
		}
		if v != caso.quien {
			t.Errorf("con %s el texto es %q y se esperaba %q", caso.proto, v, caso.quien)
		}
	}
}

// TestElStubSinCuerpoCapturaPrintInt: `printInt` es la salida de las apps que no usan TextView; sin
// cuerpo en el dex, el intérprete captura su argumento en vez de avisar.
func TestElStubSinCuerpoCapturaPrintInt(t *testing.T) {
	prog := &programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}
	m := nuevaMaquina(prog, nil)
	met := &metodoDalvik{nombre: "printInt", proto: "(I)V", retorno: "V"}
	if _, err := m.stubSinCuerpo("Lcom/myapp/Salida;", met, []any{int32(42)}); err != nil {
		t.Fatalf("printInt falló: %v", err)
	}
	if len(m.capturas) != 1 || m.capturas[0].Origen != "printInt" || m.capturas[0].Texto != "42" {
		t.Fatalf("printInt capturó %+v", m.capturas)
	}
	// y lo que no es un stub se avisa, no se finge ejecutado
	otro := &metodoDalvik{nombre: "dibuja", proto: "(I)V", retorno: "V"}
	if _, err := m.stubSinCuerpo("Lcom/myapp/Salida;", otro, []any{int32(1)}); err != nil {
		t.Fatalf("dibuja falló: %v", err)
	}
	if len(m.avisos) != 1 {
		t.Errorf("el método desconocido dejó %d avisos", len(m.avisos))
	}
}

// TestElInterpreteEjecutaElTryYAtrapaLaDivisionPorCero mide el try/catch sin vistas: la primera
// llamada no lanza nada y devuelve su valor; la segunda divide por cero, que en Dalvik es una
// ArithmeticException, y el catch del mismo método la recoge y devuelve -1. Antes esta prueba medía
// la frontera —el intérprete rechazaba la tabla por su nombre—, y esa frontera ya no existe.
func TestElInterpreteEjecutaElTryYAtrapaLaDivisionPorCero(t *testing.T) {
	const conTry = `package com.myapp;
public class Riesgo {
    static int divide(int a, int b) {
        try { return a / b; } catch (ArithmeticException e) { return -1; }
    }
    public static void main(String[] args) {
        System.out.println("resultado " + divide(4, 2));
        System.out.println("fallo " + divide(4, 0));
    }
}
`
	d, err := readDex(dexDeFuentes(t, fuenteJava{nombre: "Riesgo.java", contenido: conTry}))
	if err != nil {
		t.Fatalf("el fuente no se dexea: %v", err)
	}
	res, _ := ejecutaOnCreate(d, nil, "Lcom/myapp/Riesgo;")
	if res.Error != "" {
		t.Fatalf("el try/catch no se ejecutó: %s", res.Error)
	}
	if len(res.Consola) != 2 {
		t.Fatalf("se esperaban dos líneas de consola y hay %+v", res.Consola)
	}
	if res.Consola[0].Texto != "resultado 2" {
		t.Errorf("la primera llamada imprimió %q y 4/2 es 2", res.Consola[0].Texto)
	}
	if res.Consola[1].Texto != "fallo -1" {
		t.Errorf("la división por cero imprimió %q y el catch devuelve -1", res.Consola[1].Texto)
	}
}

// TestElInterpreteResuelveSetTextConUnRecurso: `setText(R.string.saludo)` llega al stub como su id;
// la tabla del apk es la que lo convierte en el texto, igual que el layout resuelve `@string/x`.
func TestElInterpreteResuelveSetTextConUnRecurso(t *testing.T) {
	const conRecurso = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.widget.*;

public class Recurso extends Activity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        total.setText(R.string.saludo);
    }
}
`
	r, err := rDeTabla(tablaDelContador())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	d, err := readDex(dexDeFuentes(t,
		fuenteJava{nombre: "R.java", contenido: r},
		fuenteJava{nombre: "Recurso.java", contenido: conRecurso}))
	if err != nil {
		t.Fatalf("el dex: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	tabla, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla no se relee: %v", err)
	}
	res, layoutID := ejecutaOnCreate(d, tabla, "Lcom/hola/contador/Recurso;")
	if res.Error != "" {
		t.Fatalf("la ejecución falló: %s", res.Error)
	}
	if layoutID != 0x7f020000 {
		t.Errorf("el layout es %#08x", layoutID)
	}
	if len(res.Capturas) != 1 || res.Capturas[0].Texto != "Hola desde un recurso" ||
		res.Capturas[0].ID != "id/total" {
		t.Fatalf("la captura es %+v y se esperaba el texto del recurso sobre id/total", res.Capturas)
	}
}

// TestElSwitchRechazaUnPayloadTruncado: la cabecera del intérprete promete rechazar por nombre, no
// paniquear. Un payload que declara más brazos de los que caben se dice con su error, y los dos
// payloads bien formados siguen saltando a donde toca.
func TestElSwitchRechazaUnPayloadTruncado(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)

	// pc 0, desplazamiento 4: ident en 4, tamaño en 5, primera clave en 6-7 y un destino por brazo
	packed := []uint16{
		0x2b00, 4,
		0, 0,
		0x0100, 2, // ident en 4 y dos brazos
		1, 0, // primera clave = 1
		5, 0, // destino del brazo 0
		7, 0, // destino del brazo 1
	}
	if salto, err := m.saltoDeSwitch(packed, 0, true, int32(2)); err != nil || salto != 7 {
		t.Errorf("packed-switch con la clave 2: salto %d, err %v", salto, err)
	}
	if salto, err := m.saltoDeSwitch(packed, 0, true, int32(9)); err != nil || salto != 3 {
		t.Errorf("packed-switch con una clave que no está: salto %d, err %v", salto, err)
	}

	// sparse: ident en 2, tamaño en 3, dos claves en 4-7 y dos destinos en 8-11
	sparse := []uint16{
		0x2c00, 2,
		0x0200, 2,
		1, 0, 9, 0,
		5, 0, 7, 0,
	}
	if salto, err := m.saltoDeSwitch(sparse, 0, false, int32(9)); err != nil || salto != 7 {
		t.Errorf("sparse-switch con la clave 9: salto %d, err %v", salto, err)
	}

	// el mismo packed sin el último par de destinos: declara 2 brazos y solo caben uno
	packedCortado := packed[:len(packed)-2]
	if _, err := m.saltoDeSwitch(packedCortado, 0, true, int32(2)); err == nil {
		t.Error("se aceptó un payload de packed-switch truncado")
	} else if !strings.Contains(err.Error(), "payload de packed-switch truncado") {
		t.Errorf("el error del packed-switch truncado es %q", err)
	}
	var malo *errorDexMalformado
	if !errors.As(func() error { _, e := m.saltoDeSwitch(packedCortado, 0, true, int32(2)); return e }(), &malo) {
		t.Error("el payload truncado no se marcó como dex malformado")
	}

	sparseCortado := sparse[:len(sparse)-2]
	if _, err := m.saltoDeSwitch(sparseCortado, 0, false, int32(9)); err == nil {
		t.Error("se aceptó un payload de sparse-switch truncado")
	} else if !strings.Contains(err.Error(), "payload de sparse-switch truncado") {
		t.Errorf("el error del sparse-switch truncado es %q", err)
	}

	// un ident equivocado tampoco se ejecuta
	if _, err := m.saltoDeSwitch(packed, 0, false, int32(1)); err == nil ||
		!strings.Contains(err.Error(), "apunta al payload") {
		t.Errorf("un sparse-switch sobre un payload packed dio %v", err)
	}
	// y un desplazamiento al final del cuerpo
	if _, err := m.saltoDeSwitch(packed, 0, true, int32(1)); err != nil {
		t.Errorf("el packed-switch válido falló: %v", err)
	}
	if _, err := m.saltoDeSwitch(packed[:2], 0, true, int32(1)); err == nil {
		t.Error("se aceptó un switch cuyo payload cae fuera del cuerpo")
	}
}

// fuenteCasosSwitch es una actividad con un switch en un método propio: su dex lleva un
// packed-switch real, que es lo que se corrompe para la prueba de punta a punta.
const fuenteCasosSwitch = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.widget.*;

public class Casos extends Activity {
    int valor(int x) {
        switch (x) { case 1: return 10; case 2: return 20; default: return 0; }
    }

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        total.setText("valor " + valor(2));
    }
}
`

// rompePayloadDeSwitch devuelve una copia del dex con el tamaño del payload del packed-switch del
// método dado puesto a 100: los bytes siguen leyéndose como dex, pero el payload ya no cabe.
func rompePayloadDeSwitch(t *testing.T, dex []byte, metodo string) []byte {
	t.Helper()
	d, err := readDex(dex)
	if err != nil {
		t.Fatalf("el dex no se relee: %v", err)
	}
	datos, err := d.methodsDeCadaClase()
	if err != nil {
		t.Fatalf("el class_data: %v", err)
	}
	for _, c := range d.classes {
		cd := datos[c.name]
		for _, m := range append(append([]encodedMethod{}, cd.directs...), cd.virtuals...) {
			if m.codeOff == 0 || int(m.methodIdx) >= len(d.methods) {
				continue
			}
			if d.methods[m.methodIdx].name != metodo {
				continue
			}
			item, err := d.readCodeItem(m.codeOff)
			if err != nil {
				t.Fatalf("%s.%s: %v", c.name, metodo, err)
			}
			for pc := 0; pc+1 < len(item.insns); pc++ {
				if byte(item.insns[pc]) != 0x2b {
					continue
				}
				base := pc + int(int16(item.insns[pc+1]))
				if base < 0 || base+3 >= len(item.insns) {
					t.Fatalf("el payload del switch cae fuera del cuerpo")
				}
				pos := int(m.codeOff) + 16 + 2*(base+1)
				salida := append([]byte(nil), dex...)
				if pos+2 > len(salida) {
					t.Fatalf("el tamaño del payload queda fuera del fichero")
				}
				salida[pos] = 100
				salida[pos+1] = 0
				return salida
			}
			t.Fatalf("%s no tiene packed-switch (0x2b) en el método %s", c.name, metodo)
		}
	}
	t.Fatalf("no se encontró el método %s con cuerpo", metodo)
	return nil
}

// TestElPreviewRechazaUnSwitchDePayloadRoto es la prueba de punta a punta: el opcode real 0x2b con
// el payload mal alineado hace que `preview` devuelva error controlado, con el mensaje del payload y
// sin panic. El mismo apk sin corromper captura su texto, para que se vea que lo que rompe es el
// payload y no otra cosa.
func TestElPreviewRechazaUnSwitchDePayloadRoto(t *testing.T) {
	fuentes := append(fuentesDelContador(t), fuenteJava{nombre: "Casos.java", contenido: fuenteCasosSwitch})
	dex := dexDeFuentes(t, fuentes...)
	// la lanzadora es Casos: es donde vive el switch, y así lo recorre también el comando entero
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelContador())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apkCon := func(clases []byte) []byte {
		return zipDeEntradas(t, map[string][]byte{
			"AndroidManifest.xml": manifiesto,
			"resources.arsc":      arsc,
			"res/layout/main.xml": layout,
			"classes.dex":         clases,
		})
	}

	d, err := readDex(rompePayloadDeSwitch(t, dex, "valor"))
	if err != nil {
		t.Fatalf("el dex parcheado no se relee: %v", err)
	}
	tabla, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla no se relee: %v", err)
	}
	// primero el bueno: Casos escribe «valor 20»
	dbuena, err := readDex(dex)
	if err != nil {
		t.Fatal(err)
	}
	res, _ := ejecutaOnCreate(dbuena, tabla, "Lcom/hola/contador/Casos;")
	if res.Error != "" || len(res.Capturas) != 1 || res.Capturas[0].Texto != "valor 20" {
		t.Fatalf("el dex sin corromper no capturó «valor 20»: %+v (%s)", res.Capturas, res.Error)
	}
	// y ahora el roto: error controlado, del payload y marcado como malformado
	res, _ = ejecutaOnCreate(d, tabla, "Lcom/hola/contador/Casos;")
	if res.Error == "" {
		t.Fatal("el payload truncado se ejecutó sin error")
	}
	if !strings.Contains(res.Error, "payload de packed-switch truncado") {
		t.Errorf("el error es %q y se esperaba el del payload", res.Error)
	}
	if !res.malformado {
		t.Error("el fallo del payload no se marcó como dex malformado")
	}
	if len(res.Capturas) != 0 {
		t.Errorf("se capturó texto de una ejecución que se detuvo: %+v", res.Capturas)
	}

	// el mismo apk sin corromper: el comando captura «valor 20» en el TextView
	bueno, err := modeloVistaDeAPK(apkCon(dex), "")
	if err != nil {
		t.Fatalf("el apk sin corromper se rechazó: %v", err)
	}
	if bueno.Ejecucion == nil || bueno.Ejecucion.Error != "" || len(bueno.Ejecucion.Capturas) != 1 {
		t.Fatalf("el apk sin corromper no capturó nada: %+v", bueno.Ejecucion)
	}
	if got := bueno.Raiz.Hijos[0].TextoDinamico; got != "valor 20" {
		t.Errorf("el texto dinámico del apk bueno es %q", got)
	}

	// el comando entero, con el dex roto metido en un apk, tiene que rechazarlo
	apk := apkCon(rompePayloadDeSwitch(t, dex, "valor"))
	if destino := os.Getenv("APKC_FIXTURE_MALO"); destino != "" {
		// la prueba del código de salida se hace desde la shell; sin la variable no se escribe nada
		if err := os.WriteFile(destino, apk, 0644); err != nil {
			t.Fatalf("no se pudo escribir el apk de la sonda: %v", err)
		}
	}
	if _, err := modeloVistaDeAPK(apk, ""); err == nil {
		t.Fatal("modeloVistaDeAPK aceptó un dex con el payload del switch truncado")
	} else if !strings.Contains(err.Error(), "el classes.dex no es ejecutable") ||
		!strings.Contains(err.Error(), "payload de packed-switch truncado") {
		t.Errorf("el rechazo del comando dice %q", err)
	}
	// y sin ejecutar, el layout del mismo apk sigue leyéndose: la bandera de escape no interpreta
	if _, err := modeloVistaDeAPKCon(apk, "", false); err != nil {
		t.Errorf("con --sin-ejecutar el apk se rechazó: %v", err)
	}
}

// fuenteClickDeclarativo es el patrón de los contadores de tutorial: el clic se declara en el XML
// (`android:onClick="sumar"`) y la actividad solo tiene el método. Es el caso que el emulador no
// enlazaba: `onCreate` no llama a `setOnClickListener`, así que `oyentes` salía vacío y la pestaña
// decía «Esta ejecución no registró ningún OnClickListener sobre una vista con id».
const fuenteClickDeclarativo = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
    }

    public void sumar(View v) {
        pulsos = pulsos + 1;
        total.setText("Pulsos: " + pulsos);
    }
}
`

// layoutConOnClickDeclarativo es el layout del Contador con el clic en el botón, como lo escribe
// alguien que sigue un tutorial: sin `setOnClickListener` en el fuente.
func layoutConOnClickDeclarativo() documentoAXML {
	layout := layoutDelContador()
	layout.raiz.hijos[1].attrs = append(layout.raiz.hijos[1].attrs, atributoEscribir{
		ns: "android", nombre: "onClick", id: 0x0101026f, tipo: tipoCadena, texto: "sumar",
	})
	return layout
}

// TestElPreviewEnlazaElOnClickDeclarativoDelLayout mide el caso del usuario: un apk cuyo botón
// declara `android:onClick` publica el oyente real con su id, marca el nodo y el clic ejecuta el
// método de la actividad, sin que el fuente llame a `setOnClickListener`.
func TestElPreviewEnlazaElOnClickDeclarativoDelLayout(t *testing.T) {
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutConOnClickDeclarativo())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: rContador(t)},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteClickDeclarativo}),
	})
	if destino := os.Getenv("APKC_FIXTURE_DECL"); destino != "" {
		// la sonda del §203: deja el apk en disco para medirlo con el comando de verdad
		if err := os.WriteFile(destino, apk, 0644); err != nil {
			t.Fatalf("no se pudo escribir el apk de la sonda: %v", err)
		}
	}

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk con android:onClick no se lee: %v", err)
	}
	if reposo.Ejecucion == nil || len(reposo.Ejecucion.Oyentes) != 1 ||
		reposo.Ejecucion.Oyentes[0].ID != "id/pulsar" ||
		reposo.Ejecucion.Oyentes[0].IDRef != "0x7f010000" {
		t.Fatalf("los oyentes declarativos son %+v y se esperaba id/pulsar", reposo.Ejecucion.Oyentes)
	}
	if len(reposo.Ejecucion.Avisos) != 0 {
		t.Errorf("el enlace dejó avisos: %v", reposo.Ejecucion.Avisos)
	}
	var boton *nodoVista
	for _, h := range reposo.Raiz.Hijos {
		if h.ID == "id/pulsar" {
			boton = h
		}
	}
	if boton == nil || !boton.Oyente {
		t.Fatalf("el botón con android:onClick no quedó marcado como oyente: %+v", boton)
	}
	if got := reposo.Raiz.Hijos[0].TextoDinamico; got != "" {
		t.Errorf("en reposo el texto dinámico es %q", got)
	}

	uno, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar"})
	if err != nil {
		t.Fatalf("el clic en el oyente declarativo falló: %v", err)
	}
	if got := uno.Raiz.Hijos[0].TextoDinamico; got != "Pulsos: 1" {
		t.Errorf("tras el clic el texto es %q y se esperaba «Pulsos: 1»", got)
	}
	if uno.Ejecucion.Interacciones != 1 {
		t.Errorf("las interacciones son %d", uno.Ejecucion.Interacciones)
	}

	// y por el idRef del nodo, que es lo que manda la pestaña al pulsar en el canvas
	dos, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"0x7f010000", "0x7f010000"})
	if err != nil {
		t.Fatalf("los clics por idRef fallaron: %v", err)
	}
	if got := dos.Raiz.Hijos[0].TextoDinamico; got != "Pulsos: 2" {
		t.Errorf("tras dos clics el texto es %q y se esperaba «Pulsos: 2»", got)
	}

	// un método que no existe no inventa un oyente: queda el aviso y el clic no se puede entregar
	roto := layoutDelContador()
	roto.raiz.hijos[1].attrs = append(roto.raiz.hijos[1].attrs, atributoEscribir{
		ns: "android", nombre: "onClick", id: 0x0101026f, tipo: tipoCadena, texto: "noExiste",
	})
	layoutRoto, err := escribeAXML(roto)
	if err != nil {
		t.Fatalf("el layout roto: %v", err)
	}
	apkRoto := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layoutRoto,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: rContador(t)},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteClickDeclarativo}),
	})
	sinOyente, err := modeloVistaDeAPKConPulsos(apkRoto, "", true, nil)
	if err != nil {
		t.Fatalf("el apk con el método que no existe no se lee: %v", err)
	}
	if len(sinOyente.Ejecucion.Oyentes) != 0 {
		t.Errorf("se publicó un oyente para un método que no existe: %+v", sinOyente.Ejecucion.Oyentes)
	}
	if !strings.Contains(strings.Join(sinOyente.Ejecucion.Avisos, "\n"), "noExiste") {
		t.Errorf("el aviso no nombra el método que falta: %v", sinOyente.Ejecucion.Avisos)
	}
}

// fuenteGetTextDelUsuario es el onClick que falló en el aparato del usuario, con el paquete del
// fixture: la cadena que la vista ya tenía («Pulsos: 0») se concatena con el contador, y el texto se
// lee con `getText().toString()`, no con un campo. Antes de este arreglo, `getText` no era stub y el
// onClick se paraba en «se llamó a java.lang.CharSequence.toString sobre un objeto nulo».
const fuenteGetTextDelUsuario = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    @Override
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                pulsos = pulsos + 1;
                total.setText(total.getText().toString() + ", " + pulsos);
                System.out.println("Hice click");
            }
        });
    }
}
`

// textosDeLasCapturas devuelve solo lo que escribió la app sobre una vista, en orden.
func textosDeLasCapturas(ejecucion *resultadoEjecucion) []string {
	if ejecucion == nil {
		return nil
	}
	textos := []string{}
	for _, c := range ejecucion.Capturas {
		if c.Origen == "TextView.setText" || c.Origen == "TextView.append" {
			textos = append(textos, c.Texto)
		}
	}
	return textos
}

// TestElInterpreteGetTextDevuelveElTextoActual es el fallo medido sobre el fuente del usuario:
// `total.getText().toString()` tiene que devolver el texto que la vista ya trae del layout
// («Pulsos: 0») y no "", y la segunda pulsación tiene que acumular sobre lo que dejó la primera. El
// clic no deja avisos y su `System.out.println` sigue llegando a la consola.
func TestElInterpreteGetTextDevuelveElTextoActual(t *testing.T) {
	apk := apkDelContador(t,
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteGetTextDelUsuario})

	uno, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar"})
	if err != nil {
		t.Fatalf("el clic en id/pulsar falló: %v", err)
	}
	if avisos := uno.Ejecucion.Avisos; len(avisos) != 0 {
		t.Fatalf("un clic con el fuente del usuario deja avisos: %v", avisos)
	}
	if got := textosDeLasCapturas(uno.Ejecucion); len(got) != 1 || got[0] != "Pulsos: 0, 1" {
		t.Fatalf("las escrituras del primer clic son %v y se esperaba «Pulsos: 0, 1»", got)
	}
	if len(uno.Ejecucion.Capturas) == 0 || uno.Ejecucion.Capturas[0].ID != "id/total" {
		t.Errorf("la captura no es de id/total: %+v", uno.Ejecucion.Capturas)
	}
	if len(uno.Ejecucion.Consola) != 1 || uno.Ejecucion.Consola[0].Texto != "Hice click" {
		t.Errorf("la consola del clic es %+v y se esperaba «Hice click»", uno.Ejecucion.Consola)
	}
	if uno.Raiz == nil || len(uno.Raiz.Hijos) == 0 || uno.Raiz.Hijos[0].TextoDinamico != "Pulsos: 0, 1" {
		t.Errorf("el nodo de id/total no enseña el texto nuevo: %+v", uno.Raiz)
	}

	// el segundo clic parte de lo que dejó el primero: el estado de la vista es el real, no el del
	// layout
	dos, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar", "id/pulsar"})
	if err != nil {
		t.Fatalf("los dos clics fallaron: %v", err)
	}
	if got := textosDeLasCapturas(dos.Ejecucion); len(got) != 2 || got[1] != "Pulsos: 0, 1, 2" {
		t.Fatalf("las escrituras de dos clics son %v y se esperaba la segunda «Pulsos: 0, 1, 2»", got)
	}
	if len(dos.Ejecucion.Consola) != 2 || dos.Ejecucion.Consola[1].Texto != "Hice click" {
		t.Errorf("la consola de dos clics es %+v", dos.Ejecucion.Consola)
	}
}

// maquinaDelContador arma una máquina con la tabla del Contador y una actividad, que es lo que
// necesitan los stubs que resuelven ids o se reconocen por el receptor. No hace falta dex: los
// stubs no lo miran.
func maquinaDelContador(t *testing.T) *maquinaDalvik {
	t.Helper()
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	tabla, err := leeTabla(arsc)
	if err != nil {
		t.Fatalf("la tabla no se relee: %v", err)
	}
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, tabla)
	m.actividad = &objetoDalvik{clase: "Lcom/hola/contador/MainActivity;", campos: map[string]any{}}
	return m
}

// TestLosStubsDelInventarioDevuelvenElEstadoDeLaVista es la tabla de lo que el inventario del
// intérprete dice que una app de contador usa: cada entrada es la llamada que emite el dex y lo que
// tiene que devolver. Ninguna deja aviso: o se atiende con el estado real (el texto de la vista, su
// id, su etiqueta), o está mal y el test lo dice.
func TestLosStubsDelInventarioDevuelvenElEstadoDeLaVista(t *testing.T) {
	m := maquinaDelContador(t)
	vista := m.vista(0x7f010001)
	vista.clase = "Landroid/widget/TextView;"
	vista.texto = "Pulsos: 3"

	vistaCandidatos := []string{"Landroid/widget/TextView;", "Landroid/view/View;"}
	cadenaCandidatos := []string{"Ljava/lang/String;", "Ljava/lang/CharSequence;"}
	llama := func(candidatos []string, nombre, proto string, vals []any, receptor any) any {
		t.Helper()
		v, ok, err := m.intentaStub(candidatos, nombre, proto, vals, receptor)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if !ok {
			t.Fatalf("%s no lo atiende ningún stub", nombre)
		}
		return v
	}

	texto := llama(vistaCandidatos, "getText", "()Ljava/lang/CharSequence;", nil, vista)
	devuelto, ok := texto.(*objetoDalvik)
	if !ok || devuelto == vista {
		t.Fatalf("getText devolvió %#v y tenía que ser un objeto aparte de la vista", texto)
	}
	if devuelto.clase != "Ljava/lang/String;" || devuelto.texto != "Pulsos: 3" {
		t.Errorf("getText devolvió %q de clase %q", devuelto.texto, devuelto.clase)
	}

	if got := llama(cadenaCandidatos, "toString", "()Ljava/lang/String;", nil, devuelto); got != "Pulsos: 3" {
		t.Errorf("toString del getText es %v", got)
	}
	// una cadena vacía se lee como "", no como el nombre de su clase
	vacia := &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{}}
	if got := llama(cadenaCandidatos, "toString", "()Ljava/lang/String;", nil, vacia); got != "" {
		t.Errorf("toString de una cadena vacía es %q", got)
	}
	if got := llama(cadenaCandidatos, "length", "()I", nil, devuelto); got != int32(9) {
		t.Errorf("length de «Pulsos: 3» es %v", got)
	}
	// length cuenta unidades de 16 bits: «á» es una, no dos bytes
	acentuada := &objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{}, texto: "á"}
	if got := llama(cadenaCandidatos, "length", "()I", nil, acentuada); got != int32(1) {
		t.Errorf("length de «á» es %v y tiene que ser 1", got)
	}
	if got := llama(cadenaCandidatos, "charAt", "(I)C", []any{int32(0)}, devuelto); got != int32('P') {
		t.Errorf("charAt(0) es %v", got)
	}
	if _, ok, err := m.intentaStub(cadenaCandidatos, "charAt", "(I)C", []any{int32(99)}, devuelto); err == nil || !ok {
		t.Errorf("charAt fuera de la cadena no dio error: ok=%v err=%v", ok, err)
	}
	if got := llama(cadenaCandidatos, "isEmpty", "()Z", nil, vacia); got != boolAInt(true) {
		t.Errorf("isEmpty de \"\" es %v", got)
	}
	if got := llama(cadenaCandidatos, "isEmpty", "()Z", nil, devuelto); got != boolAInt(false) {
		t.Errorf("isEmpty de «Pulsos: 3» es %v", got)
	}
	if got := llama(cadenaCandidatos, "equals", "(Ljava/lang/Object;)Z", []any{"Pulsos: 3"}, devuelto); got != boolAInt(true) {
		t.Errorf("equals con el mismo texto es %v", got)
	}
	if got := llama(cadenaCandidatos, "equals", "(Ljava/lang/Object;)Z", []any{"otra"}, devuelto); got != boolAInt(false) {
		t.Errorf("equals con otro texto es %v", got)
	}
	if got := llama(cadenaCandidatos, "equals", "(Ljava/lang/Object;)Z", []any{nil}, devuelto); got != boolAInt(false) {
		t.Errorf("equals con null es %v", got)
	}

	// getString resuelve el id del recurso contra la tabla, como setText
	idSaludo, ok := m.tabla.idDeNombre("string", "saludo")
	if !ok {
		t.Fatal("la tabla del Contador no tiene string/saludo")
	}
	saludo := llama([]string{"Lcom/hola/contador/MainActivity;"}, "getString", "(I)Ljava/lang/String;",
		[]any{int32(idSaludo)}, m.actividad)
	if obj, ok := saludo.(*objetoDalvik); !ok || obj.texto != "Hola desde un recurso" {
		t.Errorf("getString devolvió %#v", saludo)
	}

	if got := llama(vistaCandidatos, "getId", "()I", nil, vista); got != int32(0x7f010001) {
		t.Errorf("getId es %v y la vista se creó con 0x7f010001", got)
	}
	if got := llama(vistaCandidatos, "getTag", "()Ljava/lang/Object;", nil, vista); got != nil {
		t.Errorf("getTag sin etiqueta es %v y el framework devuelve null", got)
	}
	llama(vistaCandidatos, "setTag", "(Ljava/lang/Object;)V", []any{"marca"}, vista)
	if got := llama(vistaCandidatos, "getTag", "()Ljava/lang/Object;", nil, vista); got != "marca" {
		t.Errorf("getTag después de setTag es %v", got)
	}

	// TextView.append suma al texto de la vista y se captura como una escritura más
	llama(vistaCandidatos, "append", "(Ljava/lang/CharSequence;)V", []any{"!"}, vista)
	if vista.texto != "Pulsos: 3!" {
		t.Errorf("después de append el texto de la vista es %q", vista.texto)
	}
	if n := len(m.capturas); n == 0 || m.capturas[n-1].Origen != "TextView.append" ||
		m.capturas[n-1].Texto != "Pulsos: 3!" {
		t.Errorf("append no dejó su captura: %+v", m.capturas)
	}

	// el StringBuilder sigue siendo el de javac: append devuelve el mismo objeto para encadenar
	sb := &objetoDalvik{clase: "Ljava/lang/StringBuilder;", campos: map[string]any{}}
	if got := llama([]string{"Ljava/lang/StringBuilder;"}, "append", "(Ljava/lang/String;)Ljava/lang/StringBuilder;",
		[]any{"a"}, sb); got != sb {
		t.Errorf("append de StringBuilder no devuelve el mismo objeto: %#v", got)
	}
	llama([]string{"Ljava/lang/StringBuilder;"}, "append", "(I)Ljava/lang/StringBuilder;",
		[]any{int32(2)}, sb)
	if got := llama([]string{"Ljava/lang/StringBuilder;"}, "toString", "()Ljava/lang/String;", nil, sb); got != "a2" {
		t.Errorf("toString del StringBuilder es %v", got)
	}

	if len(m.avisos) != 0 {
		t.Errorf("la tabla de stubs deja avisos: %v", m.avisos)
	}

	// una cadena vacía se imprime vacía también al concatenar: el nombre de la clase no es su texto
	if got := textoDe(&objetoDalvik{clase: "Ljava/lang/String;", campos: map[string]any{}}); got != "" {
		t.Errorf("textoDe de una cadena vacía es %q", got)
	}
}

// TestLaSiembraDelLayoutNoPisaLoQueLaAppEscribio: el `android:text` del layout se copia a las vistas
// que ya existían, pero no a la que la app ya escribió, aunque lo escrito sea vacío.
func TestLaSiembraDelLayoutNoPisaLoQueLaAppEscribio(t *testing.T) {
	m := maquinaDelContador(t)
	escrita := m.vista(0x7f010001)
	m.captura("TextView.setText", escrita, "")
	m.siembraTextosDelLayout(map[uint32]string{0x7f010001: "Pulsos: 0", 0x7f010002: "Del layout"})
	if escrita.texto != "" {
		t.Errorf("la siembra pisó un setText(\"\") de la app: queda %q", escrita.texto)
	}
	if otra := m.vista(0x7f010002); otra.texto != "Del layout" {
		t.Errorf("la vista creada después de la siembra arranca en %q", otra.texto)
	}
}

// TestElInterpreteRechazaPorNombreLoQueNoEmula fija la otra mitad: lo que no tiene una aproximación
// honesta (formato de cadenas, Html, reflexión) no se stubbea a medias. El aviso lo nombra y el test
// comprueba que el nombre y el motivo están. `getEditableText` ya no está aquí: se emula como texto
// plano desde E5, porque es lo único que una app lee de un Editable.
func TestElInterpreteRechazaPorNombreLoQueNoEmula(t *testing.T) {
	m := maquinaDelContador(t)
	casos := []struct {
		nombre     string
		proto      string
		vals       []any
		candidatos []string
		receptor   any
		espera     string
	}{
		{"getString", "(I[Ljava/lang/Object;)Ljava/lang/String;", []any{int32(0x7f030000), nil},
			[]string{"Lcom/hola/contador/MainActivity;"}, m.actividad, "formato de cadenas"},
		{"fromHtml", "(Ljava/lang/String;)Landroid/text/Spanned;", []any{"<b>x</b>"},
			[]string{"Landroid/text/Html;"}, nil, "Html"},
		{"forName", "(Ljava/lang/String;)Ljava/lang/Class;", []any{"java.lang.String"},
			[]string{"Ljava/lang/Class;"}, nil, "reflexión"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m.avisos = nil
			m.vistos = map[string]bool{}
			_, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor)
			if err != nil || !ok {
				t.Fatalf("%s se rechaza con aviso, no con error: ok=%v err=%v", c.nombre, ok, err)
			}
			if len(m.avisos) != 1 || !strings.Contains(m.avisos[0], c.nombre) ||
				!strings.Contains(m.avisos[0], c.espera) {
				t.Errorf("el aviso de %s es %v y tenía que nombrarlo con %q", c.nombre, m.avisos, c.espera)
			}
		})
	}
}

// fuenteClickDeLosDos declara el clic en los dos sitios que admite Android: el `android:onClick`
// del layout y un `setOnClickListener` en `onCreate`. En el aparato gana el programático, porque el
// inflado enlaza el del XML antes y el de `onCreate` lo sustituye; el preview hacía lo contrario y
// ejecutaba el método del XML.
const fuenteClickDeLosDos = `package com.hola.contador;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    TextView total;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                total.setText("Programatico");
            }
        });
    }

    public void sumar(View v) {
        total.setText("Declarativo");
    }
}
`

// TestElPreviewOnClickProgramaticoGanaAlDelLayout fija la precedencia de Android: con el mismo
// botón declarado en el XML (`android:onClick="sumar"`, método que existe) y un
// `setOnClickListener` en `onCreate`, el clic corre el programático y el oyente publicado es uno
// solo. Antes, el oyente declarativo se enlazaba después y ganaba por ser el último.
func TestElPreviewOnClickProgramaticoGanaAlDelLayout(t *testing.T) {
	manifiesto, err := escribeAXML(manifiestoConActividad(".Casos"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutConOnClickDeclarativo())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	apk := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: rContador(t)},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteClickDeLosDos}),
	})

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk con los dos oyentes no se lee: %v", err)
	}
	if len(reposo.Ejecucion.Oyentes) != 1 || reposo.Ejecucion.Oyentes[0].ID != "id/pulsar" {
		t.Fatalf("los oyentes publicados son %+v y se esperaba solo id/pulsar", reposo.Ejecucion.Oyentes)
	}
	if avisos := reposo.Ejecucion.Avisos; len(avisos) != 0 {
		t.Errorf("la precedencia dejó avisos: %v", avisos)
	}

	uno, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/pulsar"})
	if err != nil {
		t.Fatalf("el clic falló: %v", err)
	}
	if got := uno.Raiz.Hijos[0].TextoDinamico; got != "Programatico" {
		t.Errorf("el clic corrió %q y tenía que correr el setOnClickListener del onCreate", got)
	}
	if uno.Ejecucion.Interacciones != 1 {
		t.Errorf("las interacciones son %d y se entregó un solo clic", uno.Ejecucion.Interacciones)
	}
}

// TestElPreviewNoTieneTopeDeClics fija que el emulador se porte como un dispositivo: no hay un
// número máximo de pulsaciones. Con 100 clics se entregan los 100, el texto llega a «Pulsos: 100»
// y no se avisa de ningún recorte.
func TestElPreviewNoTieneTopeDeClics(t *testing.T) {
	apk := apkDelContador(t, fuentesDelContador(t)...)
	pedidos := make([]string, 100)
	for i := range pedidos {
		pedidos[i] = "id/pulsar"
	}
	m, err := modeloVistaDeAPKConPulsos(apk, "", true, pedidos)
	if err != nil {
		t.Fatalf("los %d clics devolvieron error: %v", len(pedidos), err)
	}
	if len(m.Ejecucion.Pulsaciones) != len(pedidos) {
		t.Errorf("se entregaron %d clics de %d", len(m.Ejecucion.Pulsaciones), len(pedidos))
	}
	if m.Ejecucion.Interacciones != len(pedidos) {
		t.Errorf("las interacciones son %d y debían ser %d", m.Ejecucion.Interacciones, len(pedidos))
	}
	if avisos := strings.Join(m.Ejecucion.Avisos, "\n"); strings.Contains(avisos, "tope") {
		t.Errorf("no debería haber aviso de tope: %v", m.Ejecucion.Avisos)
	}
	if got := m.Raiz.Hijos[0].TextoDinamico; got != "Pulsos: 100" {
		t.Errorf("tras 100 clics el texto es %q", got)
	}
}

// moniValorB64 es el fixture del §358: el `.class` de `MoniValor` que sale de javac, medido en
// `$HOME/.cache/apkc-web/moni-valor-fixture/moni.b64` (sha256 del class
// 2d2b95da8d59944bdd635af4231f017dfbbb06097561661f5e8756ad68c269c5). Viaja como texto, no como
// binario (regla 2). Los tres métodos son `synchronized`, así que el dex trae `monitor-enter` y
// `monitor-exit`; el oráculo ART (`app_process64` sobre Android 14) da los mismos tres resultados.
const moniValorB64 = "yv66vgAAADQAHgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCAAIAQABeAoACgALBwAMDAANAA4BABFqYXZhL2xhbmcvSW50ZWdlcgEACHBhcnNlSW50AQAVKExqYXZhL2xhbmcvU3RyaW5nOylJBwAQAQAfamF2YS9sYW5nL051bWJlckZvcm1hdEV4Y2VwdGlvbgcAEgEACU1vbmlWYWxvcgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAxzaW5jRGV2dWVsdmUBABUoTGphdmEvbGFuZy9PYmplY3Q7KUkBAA1TdGFja01hcFRhYmxlBwAZAQATamF2YS9sYW5nL1Rocm93YWJsZQEACXNpbmNMYW56YQEADnNpbmNSZWVudnVlbHZlAQAKU291cmNlRmlsZQEADk1vbmlWYWxvci5qYXZhACEAEQACAAAAAAAEAAEABQAGAAEAEwAAAB0AAQABAAAABSq3AAGxAAAAAQAUAAAABgABAAAAAQAIABUAFgABABMAAABOAAIAAwAAAA4qWUzCECorw6xNK8MsvwACAAQACAAJAAAACQAMAAkAAAACABQAAAAGAAEAAAACABcAAAASAAH/AAkAAgcAAgcAAgABBwAYAAgAGgAWAAEAEwAAAFQAAgADAAAAFCpZTMISB7gACVcQKivDrE0rwyy/AAIABAAOAA8AAAAPABIADwAAAAIAFAAAAAYAAQAAAAMAFwAAABIAAf8ADwACBwACBwACAAEHABgACAAbABYAAQATAAAAjAACAAMAAAAjKllMwhIHuAAJVyvDpwAITSvDLL+nAAZMK7+nAAdMECqsAqwABAAEAAwADwAAAA8AEgAPAAAAAAAUABcADwAAABoAHQAPAAIAFAAAABIABAAAAAYAFAAHABoACAAhAAkAFwAAAB8ABv8ADwACBwACBwACAAEHABj6AARCBwAPAkIHAA8DAAEAHAAAAAIAHQ=="

// TestElInterpreteDecodificaLosMonitoresDeSynchronized cierra el §358: los opcodes 0x1d y 0x1e
// dejan de caer en el `default` del opcode no decodificado y se consumen como instrucciones 11x de
// una palabra, sin estado, sin avisos y sin tocar el despacho de excepciones. La guardia sobre
// `ci.insns` es imprescindible: sin los monitores en el dex, un test en verde no probaría nada.
func TestElInterpreteDecodificaLosMonitoresDeSynchronized(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(moniValorB64)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	dexBytes, err := dexDeClases([]classFull{cf})
	if err != nil {
		t.Fatalf("el dex del fixture no sale: %v", err)
	}
	d, err := readDex(dexBytes)
	if err != nil {
		t.Fatalf("el dex del fixture no se relee: %v", err)
	}

	ci, err := codeItemDeMetodo(d, "sincDevuelve")
	if err != nil {
		t.Fatalf("sincDevuelve no tiene code_item: %v", err)
	}
	entra, sale := 0, 0
	for _, w := range ci.insns {
		switch int(w & 0xff) {
		case 0x1d:
			entra++
		case 0x1e:
			sale++
		}
	}
	if entra == 0 || sale == 0 {
		t.Fatalf("el dex no trae monitores: 0x1d=%d 0x1e=%d; el test no probaría nada", entra, sale)
	}
	t.Logf("guardia: sincDevuelve trae %d monitor-enter y %d monitor-exit", entra, sale)

	prog, err := cargaPrograma(d)
	if err != nil {
		t.Fatalf("cargaPrograma del fixture: %v", err)
	}
	clase := ""
	for c := range prog.clases {
		if strings.HasSuffix(c, "/MoniValor;") || c == "LMoniValor;" {
			clase = c
		}
	}
	if clase == "" {
		for c := range prog.clases {
			t.Logf("clase disponible: %q", c)
		}
		t.Fatal("no aparece MoniValor en el dex")
	}
	m := nuevaMaquina(prog, nil)
	candado := &objetoDalvik{clase: "Ljava/lang/Object;", campos: map[string]any{}}
	llama := func(nombre string) (any, error) {
		t.Helper()
		met := m.buscaMetodo(clase, nombre, "(Ljava/lang/Object;)I")
		if met == nil {
			t.Fatalf("%s.%s no está en el programa", clase, nombre)
		}
		return m.ejecutaCuerpo(clase, met, []any{candado}, []string{"Ljava/lang/Object;"})
	}

	antes := len(m.avisos)
	v, err := llama("sincDevuelve")
	if err != nil {
		t.Fatalf("sincDevuelve falló: %v", err)
	}
	if v != int32(42) {
		t.Errorf("sincDevuelve=%v (%T), esperaba int32(42)", v, v)
	}
	if got := m.avisos[antes:]; len(got) != 0 {
		t.Errorf("sincDevuelve dejó avisos: %v", got)
	}
	t.Logf("sincDevuelve=%v", v)

	v, err = llama("sincLanza")
	if err == nil {
		t.Fatalf("sincLanza devolvió %v y debería lanzar NumberFormatException", v)
	}
	ex := excepcionDe(err)
	if ex == nil {
		t.Fatalf("sincLanza: %q no es una excepción de la app", err)
	}
	if ex.tipo != "Ljava/lang/NumberFormatException;" {
		t.Errorf("sincLanza lanzó %s, esperaba Ljava/lang/NumberFormatException;", ex.tipo)
	}
	if ex.objeto == nil || ex.objeto.mensajeExcepcion != `For input string: "x"` {
		t.Errorf("sincLanza: el mensaje es %v y debía ser For input string: \"x\"", ex.objeto)
	}
	if strings.Contains(err.Error(), "balance") {
		t.Errorf("sincLanza: %q menciona balance", err)
	}
	t.Logf("sincLanza=%q", err)

	antes = len(m.avisos)
	v, err = llama("sincReenvuelve")
	if err != nil {
		t.Fatalf("sincReenvuelve falló: %v", err)
	}
	if v != int32(42) {
		t.Errorf("sincReenvuelve=%v (%T), esperaba int32(42)", v, v)
	}
	if got := m.avisos[antes:]; len(got) != 0 {
		t.Errorf("sincReenvuelve dejó avisos: %v", got)
	}
	t.Logf("sincReenvuelve=%v", v)

	if len(m.avisos) != 0 {
		t.Errorf("los monitores dejaron %d avisos: %v", len(m.avisos), m.avisos)
	}
	t.Logf("avisos=%d", len(m.avisos))
}

// --- multidex (Fase B, §360) -----------------------------------------------------------------

// lectorDeZipAux envuelve un zip en memoria con el lector que espera dexDelApk.
func lectorDeZipAux(t *testing.T, datos []byte) *zip.Reader {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(datos), int64(len(datos)))
	if err != nil {
		t.Fatalf("el apk sintético no es un zip: %v", err)
	}
	return zr
}

// dexesDelProyecto compila las fuentes una sola vez y reparte las clases entre dos dex: el primero
// lleva las que están en `enPrimero` y el segundo el resto. Así comparten javac y la división es solo
// de empaquetado, como en un apk multidex de verdad.
func dexesDelProyecto(t *testing.T, enPrimero map[string]bool, fuentes ...fuenteJava) ([]byte, []byte) {
	t.Helper()
	emitidas, err := compilarProyectoJava(fuentes, jarDelSDK(t))
	if err != nil {
		t.Fatalf("las fuentes no compilan: %v", err)
	}
	primero := []classFull{}
	segundo := []classFull{}
	for _, c := range emitidas {
		cf, err := readClassFull(c.datos)
		if err != nil {
			t.Fatalf("%s no se lee: %v", c.interno, err)
		}
		if enPrimero[cf.Name] {
			primero = append(primero, cf)
		} else {
			segundo = append(segundo, cf)
		}
	}
	un, err := dexDeClases(primero)
	if err != nil {
		t.Fatalf("el primer dex no sale: %v", err)
	}
	dos, err := dexDeClases(segundo)
	if err != nil {
		t.Fatalf("el segundo dex no sale: %v", err)
	}
	return un, dos
}

// TestMultidexDescubreTodosLosDexEnOrdenNumerico falsa el descubrimiento: las entradas se escriben
// en un mapa, así que el directorio del zip no garantiza el orden, y `classes10.dex` va delante de
// `classes2.dex` en orden alfabético. El orden tiene que ser el numérico de ART.
func TestMultidexDescubreTodosLosDexEnOrdenNumerico(t *testing.T) {
	fuente := func(nombre string) fuenteJava {
		return fuenteJava{nombre: nombre + ".java", contenido: "package com.multidex;\npublic class " +
			nombre + " { public static int valor() { return 1; } }\n"}
	}
	un := dexDeFuentes(t, fuente("ClaseUn"))
	dos := dexDeFuentes(t, fuente("ClaseDos"))
	diez := dexDeFuentes(t, fuente("ClaseDiez"))
	apk := zipDeEntradas(t, map[string][]byte{
		"classes.dex":   un,
		"classes2.dex":  dos,
		"classes10.dex": diez,
	})
	dxs, tabla, err := dexDelApk(lectorDeZipAux(t, apk))
	if err != nil {
		t.Fatalf("dexDelApk rechazó el apk multidex: %v", err)
	}
	if len(dxs) != 3 {
		t.Fatalf("se descubrieron %d dex y se esperaban 3", len(dxs))
	}
	descriptorDe := func(b []byte) string {
		d, err := readDex(b)
		if err != nil {
			t.Fatalf("el dex sintético no se lee: %v", err)
		}
		return d.classes[0].name
	}
	esperado := []string{descriptorDe(un), descriptorDe(dos), descriptorDe(diez)}
	for i, d := range dxs {
		if tabla[esperado[i]] != d {
			t.Errorf("el dex %d no es el que contiene %s: tabla=%p, esperado=%p", i, esperado[i], tabla[esperado[i]], d)
		}
	}
}

// fuenteAuxMultidex y fuenteActividadMultidex son las dos piezas del apk sintético: el auxiliar
// vive en classes.dex, la actividad en classes2.dex y el onCreate cruza de un dex al otro.
const fuenteAuxMultidex = `package com.hola.contador;
public class Aux {
    public static int diez() { return 10; }
}
`

const fuenteActividadMultidex = `package com.hola.contador;
import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;
public class MainActivity extends Activity {
    TextView total;
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        total.setText("Hola " + Aux.diez());
    }
}
`

// apkConActividadEnElSegundoDex arma el apk con la actividad fuera del primer dex.
func apkConActividadEnElSegundoDex(t *testing.T) []byte {
	t.Helper()
	un, dos := dexesDelProyecto(t, map[string]bool{"Lcom/hola/contador/Aux;": true},
		fuenteJava{nombre: "R.java", contenido: rContador(t)},
		fuenteJava{nombre: "Aux.java", contenido: fuenteAuxMultidex},
		fuenteJava{nombre: "MainActivity.java", contenido: fuenteActividadMultidex})
	manifiesto, err := escribeAXML(manifiestoConActividad(".MainActivity"))
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	layout, err := escribeAXML(layoutDelContador())
	if err != nil {
		t.Fatalf("el layout: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelContador())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"resources.arsc":      arsc,
		"res/layout/main.xml": layout,
		"classes.dex":         un,
		"classes2.dex":        dos,
	})
}

// descriptorLanzadorDelApk deriva el descriptor de la actividad del manifiesto usando la MISMA
// función que produce el modelo (`actividadLanzadora`), sin escribir el nombre a mano. Así el test
// sigue la resolución de Fase C cuando cambie, en vez de quedar atado al nombre del fixture.
func descriptorLanzadorDelApk(t *testing.T, apk []byte) string {
	t.Helper()
	zr := lectorDeZipAux(t, apk)
	manifiesto, err := entradaDeZip(zr, "AndroidManifest.xml")
	if err != nil {
		t.Fatalf("el manifiesto del fixture: %v", err)
	}
	axml, err := leeAXML(manifiesto)
	if err != nil {
		t.Fatalf("el AXML del manifiesto: %v", err)
	}
	ctx := &contextoVista{axml: axml}
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol del manifiesto: %v", err)
	}
	actividad, ok := actividadLanzadora(arbol, ctx)
	if !ok {
		t.Fatal("el manifiesto del fixture no tiene lanzadora")
	}
	paquete := ctx.textoDeAtributo(arbol, "", "package")
	nombre := ctx.textoDeAtributo(actividad, uriDeRecursos, "name")
	return descriptorDeActividad(paquete, nombre)
}

// TestMultidexElMetodoEjecutadoEsDelSegundoDex falsa que el método que se ejecuta pertenece al dex
// donde está su clase, no al primero de la lista.
func TestMultidexElMetodoEjecutadoEsDelSegundoDex(t *testing.T) {
	apk := apkConActividadEnElSegundoDex(t)
	descriptor := descriptorLanzadorDelApk(t, apk)
	dxs, tabla, err := dexDelApk(lectorDeZipAux(t, apk))
	if err != nil {
		t.Fatalf("dexDelApk rechazó el apk: %v", err)
	}
	if len(dxs) != 2 || tabla[descriptor] != dxs[1] {
		t.Fatalf("la actividad no está en el segundo dex: %d dex, tabla=%p", len(dxs), tabla[descriptor])
	}
	res, _, err := ejecutaOnCreateConEventos(dxs, nil, descriptor, nil, nil, nil)
	if err != nil {
		t.Fatalf("la ejecución multidex devolvió error: %v", err)
	}
	if res.Error != "" || res.maquina == nil {
		t.Fatalf("la ejecución no dejó máquina: %+v", res)
	}
	met := res.maquina.buscaMetodo(descriptor, "onCreate", "(Landroid/os/Bundle;)V")
	if met == nil {
		t.Fatal("no se encontró el onCreate de la actividad")
	}
	if met.dex != dxs[1] {
		t.Fatalf("el onCreate está en el dex %p y se esperaba el segundo %p", met.dex, dxs[1])
	}
}

// TestMultidexElModeloNoRegresaDelSegundoDex falsa el criterio observable sin campo nuevo: la
// actividad del JSON es la lanzadora calculada desde el manifiesto, el ciclo pasa por onCreate y las
// capturas traen el texto real que el onCreate escribió llamando al otro dex.
func TestMultidexElModeloNoRegresaDelSegundoDex(t *testing.T) {
	apk := apkConActividadEnElSegundoDex(t)
	descriptor := descriptorLanzadorDelApk(t, apk)
	m, err := modeloVistaDeAPK(apk, "")
	if err != nil {
		t.Fatalf("el apk multidex no se deja leer: %v", err)
	}
	if m.Ejecucion == nil {
		t.Fatal("el modelo no trae ejecución")
	}
	if m.Ejecucion.Error != "" {
		t.Fatalf("la ejecución multidex falló: %s", m.Ejecucion.Error)
	}
	quiero := claseCorta(descriptor)
	if m.Ejecucion.Actividad != quiero {
		t.Errorf("la actividad es %q y se esperaba la lanzadora %q", m.Ejecucion.Actividad, quiero)
	}
	if !tienePaso(m.Ejecucion.Ciclo, quiero+".onCreate") {
		t.Errorf("el ciclo no pasó por onCreate: %v", m.Ejecucion.Ciclo)
	}
	if len(m.Ejecucion.Capturas) == 0 {
		t.Fatal("el onCreate no capturó ninguna escritura")
	}
	if m.Ejecucion.Capturas[0].Texto != "Hola 10" {
		t.Errorf("la captura es %q y se esperaba «Hola 10» del otro dex", m.Ejecucion.Capturas[0].Texto)
	}
	if m.Raiz == nil || len(m.Raiz.Hijos) == 0 || m.Raiz.Hijos[0].TextoDinamico != "Hola 10" {
		t.Errorf("el layout no recibió el texto dinámico: %+v", m.Raiz)
	}
}

// TestMultidexRechazaDescriptorDuplicado falsa la frontera: el mismo class_def en dos dex se rechaza
// con el descriptor en el mensaje, porque sus índices no se pueden mezclar.
func TestMultidexRechazaDescriptorDuplicado(t *testing.T) {
	d := dexDeFuentes(t, fuenteJava{nombre: "Dup.java", contenido: "package com.multidex;\n" +
		"public class Dup { public static int uno() { return 1; } }\n"})
	apk := zipDeEntradas(t, map[string][]byte{"classes.dex": d, "classes2.dex": d})
	_, _, err := dexDelApk(lectorDeZipAux(t, apk))
	if err == nil {
		t.Fatal("dexDelApk aceptó el mismo descriptor en dos dex")
	}
	if !strings.Contains(err.Error(), "Lcom/multidex/Dup;") || !strings.Contains(err.Error(), "duplicado") {
		t.Errorf("el error no nombra el descriptor duplicado: %q", err)
	}
}

// TestClinitPerezosoLlenaLosEstaticosYLeeElMismoVariasVeces: el `<clinit>` no se ejecuta al cargar
// el dex, sino la primera vez que se toca un estático de la clase. El cuerpo lee `base` de la propia
// clase (`veces = base + 1`): sin el guardián de reentrada esa lectura volvería a entrar en el
// `<clinit>` y no terminaría. La segunda lectura desde fuera no debe repetirlo.
func TestClinitPerezosoLlenaLosEstaticosYLeeElMismoVariasVeces(t *testing.T) {
	d := dexDeFuentes(t, fuenteJava{nombre: "Estaticos.java", contenido: "package com.ejemplo;\n" +
		"public class Estaticos {\n" +
		"  static int base = 1;\n" +
		"  static int[] grupo = new int[3];\n" +
		"  static int veces = base + 1;\n" +
		"}\n"})
	dex, err := readDex(d)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := cargaPrograma(dex)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	cl := prog.clases["Lcom/ejemplo/Estaticos;"]
	if cl == nil {
		t.Fatal("la clase Estaticos no está en el dex")
	}
	if cl.metodos["<clinit>()V"] == nil || cl.metodos["<clinit>()V"].item == nil {
		t.Fatal("el dex generado no trae el <clinit> con cuerpo que la prueba necesita")
	}
	if cl.clinitEstado != 0 {
		t.Fatalf("cargaPrograma ya ejecutó el <clinit> (estado %d)", cl.clinitEstado)
	}
	m := nuevaMaquina(prog, nil)
	grupo := dexField{owner: "Lcom/ejemplo/Estaticos;", name: "grupo", typeOf: "[I"}
	veces := dexField{owner: "Lcom/ejemplo/Estaticos;", name: "veces", typeOf: "I"}

	arr, err := m.leeEstatico(grupo)
	if err != nil {
		t.Fatalf("leer grupo: %v", err)
	}
	a, ok := arr.(*arregloDalvik)
	if !ok || len(a.datos) != 3 {
		t.Fatalf("el <clinit> no dejó el arreglo de tres huecos: %#v", arr)
	}
	if cl.clinitEstado != 2 {
		t.Errorf("tras la primera lectura el estado es %d y debiera ser 2", cl.clinitEstado)
	}
	for i := 0; i < 3; i++ {
		n, err := m.leeEstatico(veces)
		if err != nil {
			t.Fatalf("leer veces: %v", err)
		}
		if n != int32(2) {
			t.Fatalf("la lectura %d de veces dio %v y el <clinit> solo debió correr una vez", i, n)
		}
	}
	if len(m.avisos) != 0 {
		t.Errorf("el <clinit> dejó avisos: %v", m.avisos)
	}
}

// TestClinitPerezosoSinCuerpoNoSeReintenta: una clase sin `<clinit>` ejecutable se marca como
// inicializada en la primera lectura y no deja avisos; no hay cuerpo que pueda cambiar el cero.
func TestClinitPerezosoSinCuerpoNoSeReintenta(t *testing.T) {
	d := dexDeFuentes(t, fuenteJava{nombre: "Plano.java", contenido: "package com.ejemplo;\n" +
		"public class Plano { static int valor = 7; }\n"})
	dex, err := readDex(d)
	if err != nil {
		t.Fatal(err)
	}
	prog, err := cargaPrograma(dex)
	if err != nil {
		t.Fatalf("el programa no carga: %v", err)
	}
	cl := prog.clases["Lcom/ejemplo/Plano;"]
	if cl == nil {
		t.Fatal("la clase Plano no está en el dex")
	}
	m := nuevaMaquina(prog, nil)
	f := dexField{owner: "Lcom/ejemplo/Plano;", name: "valor", typeOf: "I"}
	if v, err := m.leeEstatico(f); err != nil || v != int32(7) {
		t.Fatalf("el valor codificado del estático no se leyó: %v, %v", v, err)
	}
	if cl.clinitEstado != 2 {
		t.Errorf("la clase sin <clinit> quedó en estado %d", cl.clinitEstado)
	}
	if len(m.avisos) != 0 {
		t.Errorf("la clase sin <clinit> dejó avisos: %v", m.avisos)
	}
}

// ejecutaFillArrayData arma un método con `new-array` + `fill-array-data` + `return-object` sobre un
// dex sintético (el emisor del fork no genera este opcode: escribe la cadena dup/astore) y devuelve
// el arreglo que dejó el payload. El payload empieza en la unidad 6, tres palabras después del
// opcode, y los literales van en orden little-endian como en un dex de d8.
func ejecutaFillArrayData(t *testing.T, tipo string, cuenta uint16, payload []uint16) *arregloDalvik {
	t.Helper()
	d := &dexFile{types: []string{"", tipo}}
	insns := append([]uint16{
		0x0012 | cuenta<<12, // const/4 v0, cuenta
		0x0023, 1, // new-array v0, v0, tipo
		0x0026, 3, // fill-array-data v0, +3 (payload en la unidad 6)
		0x0011, // return-object v0
	}, payload...)
	met := &metodoDalvik{nombre: "rellena", retorno: tipo, proto: "()" + tipo, dex: d,
		item: &codeItem{registersSize: 1, insnsSize: uint32(len(insns)), insns: insns}}
	m := nuevaMaquina(&programaDalvik{dex: d, clases: map[string]*claseDalvik{}}, nil)
	v, err := m.ejecutaCuerpo("Lcom/ejemplo/A;", met, nil, nil)
	if err != nil {
		t.Fatalf("el método con fill-array-data falló: %v", err)
	}
	arr, ok := v.(*arregloDalvik)
	if !ok {
		t.Fatalf("el método devolvió %T y se esperaba un arreglo", v)
	}
	return arr
}

// TestFillArrayDataLlenaLosArreglosConLosLiteralesDelPayload mide los cuatro anchos sobre los tipos
// que el intérprete guarda de distinta forma. Los valores son los del payload, sin inventar ninguno.
func TestFillArrayDataLlenaLosArreglosConLosLiteralesDelPayload(t *testing.T) {
	casos := []struct {
		nombre  string
		tipo    string
		cuenta  uint16
		payload []uint16
		quiero  []any
	}{
		{
			nombre:  "long",
			tipo:    "[J",
			cuenta:  2,
			payload: []uint16{0x0300, 8, 2, 0, 0x0708, 0x0506, 0x0304, 0x0102, 0xffff, 0xffff, 0xffff, 0xffff},
			quiero:  []any{int64(0x0102030405060708), int64(-1)},
		},
		{
			nombre:  "byte con signo",
			tipo:    "[B",
			cuenta:  4,
			payload: []uint16{0x0300, 1, 4, 0, 0xfe01, 0x807f},
			quiero:  []any{int32(1), int32(-2), int32(127), int32(-128)},
		},
		{
			nombre:  "int",
			tipo:    "[I",
			cuenta:  2,
			payload: []uint16{0x0300, 4, 2, 0, 0x3344, 0x1122, 0xfff9, 0xffff},
			quiero:  []any{int32(0x11223344), int32(-7)},
		},
		{
			nombre:  "char sin signo",
			tipo:    "[C",
			cuenta:  2,
			payload: []uint16{0x0300, 2, 2, 0, 0x0061, 0x0062},
			quiero:  []any{int32('a'), int32('b')},
		},
		{
			nombre:  "boolean",
			tipo:    "[Z",
			cuenta:  2,
			payload: []uint16{0x0300, 1, 2, 0, 0x0100},
			quiero:  []any{int32(0), int32(1)},
		},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			arr := ejecutaFillArrayData(t, caso.tipo, caso.cuenta, caso.payload)
			if len(arr.datos) != len(caso.quiero) {
				t.Fatalf("el arreglo tiene %d huecos y se esperaban %d", len(arr.datos), len(caso.quiero))
			}
			for i, quiero := range caso.quiero {
				if arr.datos[i] != quiero {
					t.Errorf("el elemento %d es %#v (%T) y se esperaba %#v (%T)",
						i, arr.datos[i], arr.datos[i], quiero, quiero)
				}
			}
		})
	}
}

// TestFillArrayDataRechazaUnPayloadMalformado: un ident ajeno, un ancho que no existe, un payload
// cortado o más elementos que huecos deben fallar sin escribir nada del arreglo.
func TestFillArrayDataRechazaUnPayloadMalformado(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	casos := []struct {
		nombre  string
		payload []uint16
		quiere  string
	}{
		{"ident de packed-switch", []uint16{0x0100, 8, 2, 0, 0, 0, 0, 0}, "no a 0x0300"},
		{"ancho imposible", []uint16{0x0300, 3, 2, 0, 0, 0, 0}, "ancho de 3 bytes"},
		{"payload cortado", []uint16{0x0300, 8, 2, 0, 0, 0, 0, 0}, "truncado"},
		{"más elementos que huecos", []uint16{0x0300, 1, 3, 0, 0, 0}, "sobre un arreglo de 2"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			arr := &arregloDalvik{tipo: "[J", datos: make([]any, 2)}
			insns := append([]uint16{0x0026, 3, 0x0011}, caso.payload...)
			err := m.rellenaDesdeArrayData(insns, 0, arr)
			if err == nil || !strings.Contains(err.Error(), caso.quiere) {
				t.Fatalf("el payload malformado dio %v y se esperaba «%s»", err, caso.quiere)
			}
			for i, v := range arr.datos {
				if v != nil {
					t.Errorf("el elemento %d se escribió antes de rechazar el payload: %#v", i, v)
				}
			}
		})
	}
}

// TestStubsDeEnteroYMatematicasCalculanLosLiterales: el `<clinit>` de `ScatterMapKt` construye un
// `MutableScatterMap` vacío y su `loadedCapacity` llama a estas dos funciones puras del JDK. Sus
// resultados son los de la definición, no un cero inventado para acallar el aviso.
func TestStubsDeEnteroYMatematicasCalculanLosLiterales(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	casos := []struct {
		nombre string
		proto  string
		clase  string
		vals   []any
		quiero int32
	}{
		{"numberOfLeadingZeros", "(I)I", "Ljava/lang/Integer;", []any{int32(1)}, 31},
		{"numberOfLeadingZeros", "(I)I", "Ljava/lang/Integer;", []any{int32(0)}, 32},
		{"numberOfLeadingZeros", "(I)I", "Ljava/lang/Integer;", []any{int32(-1)}, 0},
		{"max", "(II)I", "Ljava/lang/Math;", []any{int32(-3), int32(7)}, 7},
		{"max", "(II)I", "Ljava/lang/Math;", []any{int32(9), int32(2)}, 9},
	}
	for _, caso := range casos {
		v, ok, err := m.intentaStub([]string{caso.clase}, caso.nombre, caso.proto, caso.vals, nil)
		if err != nil || !ok {
			t.Fatalf("%s%v no se resolvió: ok=%v, err=%v", caso.nombre, caso.proto, ok, err)
		}
		if v != caso.quiero {
			t.Errorf("%s%v dio %v y se esperaba %d", caso.nombre, caso.proto, v, caso.quiero)
		}
	}
}

// TestStubArraysFillEscribeSoloElRangoPedido: es el paso que falta para que el `<clinit>` de
// `ScatterMapKt` termine de inicializar los metadatos del mapa vacío.
func TestStubArraysFillEscribeSoloElRangoPedido(t *testing.T) {
	m := nuevaMaquina(&programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}, nil)
	arr := &arregloDalvik{tipo: "[J", datos: make([]any, 4)}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Arrays;"}, "fill", "([JIIJ)V",
		[]any{arr, int32(1), int32(3), int64(-7)}, nil)
	if err != nil || !ok || v != nil {
		t.Fatalf("Arrays.fill no se resolvió: ok=%v, err=%v, valor=%v", ok, err, v)
	}
	for i, quiero := range []any{nil, int64(-7), int64(-7), nil} {
		if arr.datos[i] != quiero {
			t.Errorf("el hueco %d es %#v y se esperaba %#v", i, arr.datos[i], quiero)
		}
	}
	if _, _, err := m.intentaStub([]string{"Ljava/util/Arrays;"}, "fill", "([JIIJ)V",
		[]any{arr, int32(2), int32(1), int64(0)}, nil); err == nil {
		t.Error("se aceptó un rango invertido")
	}
}
