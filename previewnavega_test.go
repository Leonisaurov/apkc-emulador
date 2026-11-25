package main

// La prueba de aceptación de E1 del plan de emulación: ciclo de vida y navegación entre dos
// actividades propias. El APK se arma en memoria con los escritores del fork y el dex sale del
// emisor del fork, como el resto de las pruebas de la vista previa (regla 2: sin binarios
// versionados).

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// tablaDelNavega: dos layouts y los cuatro ids que usan las dos actividades.
func tablaDelNavega() tablaEscribir {
	return tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.hola.navega",
		global:        []string{"res/layout/main.xml", "res/layout/segunda.xml"},
		globalUTF8:    true,
		nombresTipo:   []string{"id", "layout"},
		nombresUTF8:   false,
		claves:        []string{"abrir", "texto", "total", "volver", "main", "segunda"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{
				id:        1,
				specFlags: []uint32{0, 0, 0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 0, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 1, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 2, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
					{clave: 3, flags: 0x0004, valor: valorEscribir{tipo: 0x12, datos: 0}},
				}}},
			},
			{
				id:        2,
				specFlags: []uint32{0, 0},
				bloques: []bloqueEscribir{{entradas: []entradaEscribir{
					{clave: 4, valor: valorEscribir{tipo: 0x03, datos: 0}},
					{clave: 5, valor: valorEscribir{tipo: 0x03, datos: 1}},
				}}},
			},
		},
	}
}

// layoutDeUnaPantalla arma un LinearLayout vertical con un TextView y un Button, que es todo lo que
// necesitan las dos pantallas de la prueba.
func layoutDeUnaPantalla(idTexto uint32, texto string, idBoton uint32, etiqueta string) documentoAXML {
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
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: idTexto},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: texto},
					},
				},
				{
					nombre: "Button",
					linea:  14,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "id", id: 0x010100d0, tipo: tipoReferencia, datos: idBoton},
						{ns: "android", nombre: "layout_width", id: 0x010100f4, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "layout_height", id: 0x010100f5, tipo: tipoEnteroDec, datos: 0xfffffffe},
						{ns: "android", nombre: "text", id: 0x0101014f, tipo: tipoCadena, texto: etiqueta},
					},
				},
			},
		},
	}
}

// manifiestoDelNavega declara las dos actividades: la primera con el filtro LAUNCHER y la segunda
// solo con su nombre, que es lo que `startActivity` exige para poder abrirla.
func manifiestoDelNavega() documentoAXML {
	actividad := func(nombre string, lanzadora bool) *elementoEscribir {
		hijos := []*elementoEscribir{}
		if lanzadora {
			hijos = append(hijos, &elementoEscribir{
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
			})
		}
		return &elementoEscribir{
			nombre: "activity",
			linea:  11,
			attrs: []atributoEscribir{
				{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: nombre},
			},
			hijos: hijos,
		}
	}
	return documentoAXML{
		ns:   []declaracionNS{{prefijo: "android", uri: uriAndroid, linea: 1}},
		utf8: false,
		raiz: &elementoEscribir{
			nombre: "manifest",
			linea:  1,
			attrs: []atributoEscribir{
				{nombre: "package", tipo: tipoCadena, texto: "com.hola.navega"},
				{ns: "android", nombre: "versionName", id: 0x0101021c, tipo: tipoCadena, texto: "0.1"},
			},
			hijos: []*elementoEscribir{
				{
					nombre: "application",
					linea:  9,
					attrs: []atributoEscribir{
						{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "Navega"},
					},
					hijos: []*elementoEscribir{
						actividad(".MainActivity", true),
						actividad(".SegundaActivity", false),
					},
				},
			},
		},
	}
}

const fuenteMainNavega = `package com.hola.navega;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    int pulsos = 0;
    TextView total;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        findViewById(R.id.abrir).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                abrir();
            }
        });
    }

    void abrir() {
        pulsos = pulsos + 1;
        total.setText("Pulsos: " + pulsos);
        startActivity(new Intent(this, SegundaActivity.class));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
    public void onRestart() { }
}
`

const fuenteSegundaNavega = `package com.hola.navega;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class SegundaActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.segunda);
        TextView texto = (TextView) findViewById(R.id.texto);
        texto.setText("Pantalla 2");
        findViewById(R.id.volver).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                finish();
            }
        });
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
`

// apkDelNavega arma el APK de dos pantallas: manifiesto con las dos actividades, tabla, los dos
// layouts y el dex de las dos clases con su R.java generado desde la tabla.
func apkDelNavega(t *testing.T) []byte {
	t.Helper()
	return apkDelNavegaConManifiesto(t, manifiestoDelNavega())
}

// apkDelNavegaConManifiesto es el mismo APK con otro manifiesto: lo usa la prueba de la frontera
// para dejar la segunda actividad fuera de la declaración sin cambiar el dex.
func apkDelNavegaConManifiesto(t *testing.T, manifiestoDoc documentoAXML) []byte {
	t.Helper()
	return apkDelNavegaConSegunda(t, manifiestoDoc, fuenteSegundaNavega)
}

// apkDelNavegaConSegunda es el mismo APK con otra fuente para la segunda actividad: lo usan las
// pruebas de la auditoría E1.1 (la pantalla que no pide layout y la navegación circular).
func apkDelNavegaConSegunda(t *testing.T, manifiestoDoc documentoAXML, fuenteSegunda string) []byte {
	t.Helper()
	return apkDelNavegaCon(t, manifiestoDoc, fuenteMainNavega, fuenteSegunda)
}

// apkDelNavegaCon es el mismo APK con las dos fuentes de actividad que pida la prueba.
func apkDelNavegaCon(t *testing.T, manifiestoDoc documentoAXML, fuenteMain, fuenteSegunda string) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDoc)
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	main, err := escribeAXML(layoutDeUnaPantalla(0x7f010002, "Pulsos: 0", 0x7f010000, "Abrir"))
	if err != nil {
		t.Fatalf("el layout main: %v", err)
	}
	segunda, err := escribeAXML(layoutDeUnaPantalla(0x7f010001, "Segunda", 0x7f010003, "Volver"))
	if err != nil {
		t.Fatalf("el layout segunda: %v", err)
	}
	arsc, err := escribeTablaRecursos(tablaDelNavega())
	if err != nil {
		t.Fatalf("la tabla: %v", err)
	}
	r, err := rDeTabla(tablaDelNavega())
	if err != nil {
		t.Fatalf("el R.java: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml":    manifiesto,
		"resources.arsc":         arsc,
		"res/layout/main.xml":    main,
		"res/layout/segunda.xml": segunda,
		"classes.dex": dexDeFuentes(t,
			fuenteJava{nombre: "R.java", contenido: r},
			fuenteJava{nombre: "MainActivity.java", contenido: fuenteMain},
			fuenteJava{nombre: "SegundaActivity.java", contenido: fuenteSegunda}),
	})
}

func tienePaso(pasos []string, buscado string) bool {
	for _, p := range pasos {
		if p == buscado {
			return true
		}
	}
	return false
}

// TestElPreviewNavegaEntreActividadesPropias es el criterio de aceptación de E1: sin clics se ve la
// lanzadora; un clic que llama a `startActivity` deja delante la segunda pantalla con su layout y su
// actividad; y `finish()` vuelve a la primera conservando lo que ya había cambiado (el texto sigue
// en «Pulsos: 1», no vuelve al «Pulsos: 0» del layout), que es lo que distingue conservar la
// máquina de repetir su `onCreate`.
func TestElPreviewNavegaEntreActividadesPropias(t *testing.T) {
	apk := apkDelNavega(t)

	reposo, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk de dos pantallas no se lee: %v", err)
	}
	if reposo.Ejecucion == nil || reposo.Ejecucion.Error != "" {
		t.Fatalf("la ejecución en reposo falló: %+v", reposo.Ejecucion)
	}
	if reposo.Layout != "res/layout/main.xml" {
		t.Errorf("sin clics el layout es %q y se esperaba el main", reposo.Layout)
	}
	if len(reposo.Ejecucion.Pantallas) != 1 {
		t.Errorf("sin clics el recorrido es %v", reposo.Ejecucion.Pantallas)
	}
	for _, esperado := range []string{"com.hola.navega.MainActivity.onCreate",
		"com.hola.navega.MainActivity.onStart", "com.hola.navega.MainActivity.onResume"} {
		if !tienePaso(reposo.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo no pasó por %s: %v", esperado, reposo.Ejecucion.Ciclo)
		}
	}

	una, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir"})
	if err != nil {
		t.Fatalf("el clic que navega falló: %v", err)
	}
	if una.Layout != "res/layout/segunda.xml" {
		t.Fatalf("tras navegar el layout es %q y se esperaba el de la segunda pantalla; capturas=%+v avisos=%v",
			una.Layout, una.Ejecucion.Capturas, una.Ejecucion.Avisos)
	}
	if una.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("tras navegar la actividad es %q", una.Actividad)
	}
	if una.Ejecucion == nil || una.Ejecucion.Actividad != "com.hola.navega.SegundaActivity" {
		t.Errorf("la ejecución no dice que corre la segunda actividad: %+v", una.Ejecucion)
	}
	if len(una.Ejecucion.Pantallas) != 2 ||
		una.Ejecucion.Pantallas[1] != "com.hola.navega.SegundaActivity" {
		t.Errorf("el recorrido tras navegar es %v", una.Ejecucion.Pantallas)
	}
	if una.Raiz == nil || len(una.Raiz.Hijos) != 2 ||
		una.Raiz.Hijos[0].TextoDinamico != "Pantalla 2" {
		t.Errorf("la segunda pantalla no pinta su texto: %+v", una.Raiz)
	}
	for _, esperado := range []string{
		"com.hola.navega.MainActivity.onPause", "com.hola.navega.MainActivity.onStop",
		"com.hola.navega.SegundaActivity.onCreate", "com.hola.navega.SegundaActivity.onStart",
		"com.hola.navega.SegundaActivity.onResume",
	} {
		if !tienePaso(una.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo tras navegar no pasó por %s: %v", esperado, una.Ejecucion.Ciclo)
		}
	}

	vuelta, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir", "id/volver"})
	if err != nil {
		t.Fatalf("el clic que vuelve falló: %v", err)
	}
	if vuelta.Layout != "res/layout/main.xml" {
		t.Fatalf("tras finish() el layout es %q y se esperaba el main", vuelta.Layout)
	}
	if vuelta.Actividad != "com.hola.navega.MainActivity" {
		t.Errorf("tras finish() la actividad es %q", vuelta.Actividad)
	}
	if len(vuelta.Ejecucion.Pantallas) != 3 ||
		vuelta.Ejecucion.Pantallas[2] != "com.hola.navega.MainActivity" {
		t.Errorf("el recorrido tras volver es %v", vuelta.Ejecucion.Pantallas)
	}
	if vuelta.Raiz == nil || len(vuelta.Raiz.Hijos) != 2 {
		t.Fatalf("la pantalla de vuelta no tiene sus dos hijos: %+v", vuelta.Raiz)
	}
	texto := vuelta.Raiz.Hijos[0].TextoDinamico
	if texto != "Pulsos: 1" {
		t.Errorf("al volver el texto es %q y se esperaba «Pulsos: 1»: el estado no se conservó", texto)
	}
	for _, esperado := range []string{
		"com.hola.navega.SegundaActivity.onPause", "com.hola.navega.SegundaActivity.onStop",
		"com.hola.navega.SegundaActivity.onDestroy",
		"com.hola.navega.MainActivity.onRestart", "com.hola.navega.MainActivity.onResume",
	} {
		if !tienePaso(vuelta.Ejecucion.Ciclo, esperado) {
			t.Errorf("el ciclo al volver no pasó por %s: %v", esperado, vuelta.Ejecucion.Ciclo)
		}
	}
	// el ciclo compartido va en orden real, no agrupado por pantalla: la pausa de la primera va
	// antes del onCreate de la segunda, y el reinicio de la primera va después del destroy
	orden := []string{
		"com.hola.navega.MainActivity.onCreate", "com.hola.navega.MainActivity.onStart",
		"com.hola.navega.MainActivity.onResume",
		"com.hola.navega.MainActivity.onPause", "com.hola.navega.MainActivity.onStop",
		"com.hola.navega.SegundaActivity.onCreate", "com.hola.navega.SegundaActivity.onStart",
		"com.hola.navega.SegundaActivity.onResume",
		"com.hola.navega.SegundaActivity.onPause", "com.hola.navega.SegundaActivity.onStop",
		"com.hola.navega.SegundaActivity.onDestroy",
		"com.hola.navega.MainActivity.onRestart", "com.hola.navega.MainActivity.onStart",
		"com.hola.navega.MainActivity.onResume",
	}
	if strings.Join(vuelta.Ejecucion.Ciclo, "\n") != strings.Join(orden, "\n") {
		t.Errorf("el ciclo no va en orden real:\n%v\ny se esperaba\n%v", vuelta.Ejecucion.Ciclo, orden)
	}
	if strings.Contains(strings.Join(vuelta.Ejecucion.Avisos, "\n"), "no está declarada") {
		t.Errorf("la navegación entre actividades declaradas dejó un aviso: %v", vuelta.Ejecucion.Avisos)
	}
}

// manifiestoSoloConMain deja la segunda actividad fuera del manifiesto sin tocar el dex: es la
// frontera que E1 promete rechazar por nombre.
func manifiestoSoloConMain() documentoAXML {
	m := manifiestoDelNavega()
	m.raiz.hijos[0].hijos = m.raiz.hijos[0].hijos[:1]
	return m
}

// TestElPreviewNoAbreUnaActividadNoDeclarada comprueba que `startActivity` a una clase que existe
// en el dex pero no está declarada en el manifiesto deja el aviso y no cambia de pantalla: es la
// misma regla del aparato, que no puede arrancar una actividad que el manifiesto no declara.
func TestElPreviewNoAbreUnaActividadNoDeclarada(t *testing.T) {
	apk := apkDelNavegaConManifiesto(t, manifiestoSoloConMain())
	m, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir"})
	if err != nil {
		t.Fatalf("el clic falló: %v", err)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("se cambió de pantalla a %q con la actividad sin declarar", m.Layout)
	}
	if len(m.Ejecucion.Pantallas) != 1 {
		t.Errorf("el recorrido abrió una actividad no declarada: %v", m.Ejecucion.Pantallas)
	}
	avisos := strings.Join(m.Ejecucion.Avisos, "\n")
	if !strings.Contains(avisos, "no está declarada en el manifiesto") {
		t.Errorf("falta el aviso de actividad no declarada: %v", m.Ejecucion.Avisos)
	}
}

// fuenteSegundaSinLayout es una segunda actividad que no llama a `setContentView`: en el aparato se
// queda en blanco. El preview caía al layout `main` de la lanzadora y pintaba una pantalla que la
// app no pidió, sin avisar.
const fuenteSegundaSinLayout = `package com.hola.navega;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class SegundaActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
    }

    public void onStart() { }
    public void onResume() { }
}
`

// TestElPreviewRechazaUnaActividadSinSetContentView cierra el 3.1 de la auditoría: al abrir una
// actividad que no pide layout, el preview avisa y se queda en la anterior, en vez de enseñar el
// `main` de la lanzadora como si fuera la pantalla nueva. El clic sí se entrega, así que el comando
// no devuelve error: la navegación es lo que se rechaza, no el clic.
func TestElPreviewRechazaUnaActividadSinSetContentView(t *testing.T) {
	apk := apkDelNavegaConSegunda(t, manifiestoDelNavega(), fuenteSegundaSinLayout)
	m, err := modeloVistaDeAPKConPulsos(apk, "", true, []string{"id/abrir"})
	if err != nil {
		t.Fatalf("el clic que abre la actividad sin layout devolvió error: %v", err)
	}
	if m.Layout != "res/layout/main.xml" {
		t.Errorf("se pintó el layout %q y la actividad abierta no pidió ninguno", m.Layout)
	}
	if m.Actividad == "com.hola.navega.SegundaActivity" {
		t.Errorf("la pantalla que queda delante es la segunda actividad, que no pidió layout")
	}
	if m.Ejecucion.Actividad != "com.hola.navega.MainActivity" {
		t.Errorf("la ejecución dice que corre %q y se esperaba la lanzadora", m.Ejecucion.Actividad)
	}
	if len(m.Ejecucion.Pantallas) != 1 {
		t.Errorf("el recorrido abrió una pantalla sin layout: %v", m.Ejecucion.Pantallas)
	}
	avisos := strings.Join(m.Ejecucion.Avisos, "\n")
	if !strings.Contains(avisos, "no llamó a setContentView") {
		t.Errorf("falta el aviso de la actividad sin layout: %v", m.Ejecucion.Avisos)
	}
	// la anterior vuelve a estar delante: se pausó para abrir y se reanuda al rechazar
	if !tienePaso(m.Ejecucion.Ciclo, "com.hola.navega.MainActivity.onResume") {
		t.Errorf("no se reanudó la actividad anterior: %v", m.Ejecucion.Ciclo)
	}
}

// fuenteMainCircular abre la segunda en su `onCreate`, sin esperar a un clic.
const fuenteMainCircular = `package com.hola.navega;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class MainActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        startActivity(new Intent(this, SegundaActivity.class));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
`

// fuenteSegundaCircular vuelve a la primera en su `onCreate`: con la primera forma un ciclo.
const fuenteSegundaCircular = `package com.hola.navega;

import android.app.*;
import android.content.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class SegundaActivity extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.segunda);
        startActivity(new Intent(this, MainActivity.class));
    }

    public void onStart() { }
    public void onResume() { }
    public void onPause() { }
    public void onStop() { }
    public void onDestroy() { }
}
`

// TestElPreviewTopeDeNavegacionCircular fija el tope simétrico del bucle de pantallas: dos
// actividades que se abren la una a la otra en su `onCreate` encadenarían pantallas sin fin. Al
// llegar al tope se avisa y queda delante la última que arrancó, en vez de colgar el preview.
func TestElPreviewTopeDeNavegacionCircular(t *testing.T) {
	apk := apkDelNavegaCon(t, manifiestoDelNavega(), fuenteMainCircular, fuenteSegundaCircular)
	m, err := modeloVistaDeAPKConPulsos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("la navegación circular devolvió error: %v", err)
	}
	if len(m.Ejecucion.Pantallas) != pantallasMaximas {
		t.Errorf("el recorrido se paró en %d pantallas y el tope es %d",
			len(m.Ejecucion.Pantallas), pantallasMaximas)
	}
	tope := fmt.Sprintf("tope de %d pantallas", pantallasMaximas)
	if !strings.Contains(strings.Join(m.Ejecucion.Avisos, "\n"), tope) {
		t.Errorf("el ciclo no se avisó con %q: %v", tope, m.Ejecucion.Avisos)
	}
	if m.Ejecucion.Error != "" {
		t.Errorf("el tope dejó un error de ejecución: %s", m.Ejecucion.Error)
	}
}
// Las cinco reglas del §359: la lanzadora se decide por el manifiesto tipado, se sigue hasta la
// actividad real y el layout sale del id de `setContentView` o del nombre de esa actividad. Cuando
// no hay XML, el modelo publica el centinela y un marco con la etiqueta, sin inventar pantalla.
func contextoDeLanzadoraDePrueba() *contextoVista {
	return &contextoVista{axml: &axml{tabla: tablaCadenas{}}}
}

func cadenaDePrueba(ctx *contextoVista, texto string) valor {
	for i, s := range ctx.axml.tabla.cadenas {
		if s == texto {
			return valor{tipo: tipoCadena, datos: uint32(i)}
		}
	}
	ctx.axml.tabla.cadenas = append(ctx.axml.tabla.cadenas, texto)
	return valor{tipo: tipoCadena, datos: uint32(len(ctx.axml.tabla.cadenas) - 1)}
}

func atributoTextoDePrueba(ctx *contextoVista, ns, nombre, texto string) atributoLeido {
	return atributoLeido{ns: ns, nombre: nombre, valor: cadenaDePrueba(ctx, texto)}
}

func atributoBoolDePrueba(nombre string, activo bool) atributoLeido {
	v := valor{tipo: tipoBooleano}
	if activo {
		v.datos = 1
	}
	return atributoLeido{ns: uriDeRecursos, nombre: nombre, valor: v}
}

func elementoDePrueba(nombre string, piezas ...any) *elementoLeido {
	e := &elementoLeido{nombre: nombre}
	for _, p := range piezas {
		switch v := p.(type) {
		case atributoLeido:
			e.attrs = append(e.attrs, v)
		case *elementoLeido:
			e.hijos = append(e.hijos, v)
		default:
			panic("pieza desconocida en el elemento de prueba")
		}
	}
	return e
}

func filtroDePrueba(ctx *contextoVista, acciones, categorias []string) *elementoLeido {
	f := &elementoLeido{nombre: "intent-filter"}
	for _, a := range acciones {
		f.hijos = append(f.hijos, elementoDePrueba("action",
			atributoTextoDePrueba(ctx, uriDeRecursos, "name", a)))
	}
	for _, c := range categorias {
		f.hijos = append(f.hijos, elementoDePrueba("category",
			atributoTextoDePrueba(ctx, uriDeRecursos, "name", c)))
	}
	return f
}

func manifiestoCon(ctx *contextoVista, hijos ...*elementoLeido) *elementoLeido {
	return &elementoLeido{nombre: "manifest",
		attrs: []atributoLeido{{nombre: "package", valor: cadenaDePrueba(ctx, "org.fdroid.fdroid")}},
		hijos: []*elementoLeido{{nombre: "application", hijos: hijos}}}
}

// TestLanzadoraNoEsAliasDeshabilitado falsifica la regla 1: un alias con `enabled=false` tipado no
// puede abrir la app, aunque traiga MAIN+LAUNCHER. El alias habilitado sí, y se sigue hasta su
// `targetActivity` (regla 2), en vez de quedarse con el nombre del alias.
func TestLanzadoraNoEsAliasDeshabilitado(t *testing.T) {
	ctx := contextoDeLanzadoraDePrueba()
	main := filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
		[]string{"android.intent.category.LEANBACK_LAUNCHER"})
	real := elementoDePrueba("activity",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.MainActivity"), main)
	deshabilitado := elementoDePrueba("activity-alias",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.IconCalculatorActivity"),
		atributoTextoDePrueba(ctx, uriDeRecursos, "targetActivity", "org.fdroid.MainActivity"),
		atributoBoolDePrueba("enabled", false),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LAUNCHER"}))
	habilitado := elementoDePrueba("activity-alias",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.IconActivity"),
		atributoTextoDePrueba(ctx, uriDeRecursos, "targetActivity", "org.fdroid.MainActivity"),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LAUNCHER"}))
	manifiesto := manifiestoCon(ctx, real, deshabilitado, habilitado)

	if esLanzadora(deshabilitado, ctx) {
		t.Fatal("el alias con enabled=false tipado sigue siendo lanzador")
	}
	if !esLanzadora(habilitado, ctx) {
		t.Fatal("el alias habilitado por omisión no es lanzador")
	}
	elegida, ok := actividadLanzadora(manifiesto, ctx)
	if !ok {
		t.Fatal("no se encontró lanzadora")
	}
	if elegida != real {
		t.Fatalf("la lanzadora es %s (%s) y tenía que ser la actividad real a la que apunta el alias",
			ctx.textoDeAtributo(elegida, uriDeRecursos, "name"), elegida.nombre)
	}
	if elegida.nombre != "activity" {
		t.Fatalf("se eligió un %s en vez de la actividad real", elegida.nombre)
	}
}

// TestLanzadoraEnabledSoloEsBooleano falsifica la comparación de texto: `s != "false"` descartaría
// un alias cuyo `enabled` es una cadena "false", cuando el valor tipado dice que no es booleano.
func TestLanzadoraEnabledSoloEsBooleano(t *testing.T) {
	ctx := contextoDeLanzadoraDePrueba()
	alias := elementoDePrueba("activity-alias",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.IconActivity"),
		atributoTextoDePrueba(ctx, uriDeRecursos, "enabled", "false"),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LAUNCHER"}))
	if !esLanzadora(alias, ctx) {
		t.Fatal("un enabled escrito como cadena se trató como booleano falso")
	}
}

// TestLanzadoraPriorizaActividadConDefault falsifica la regla 3: MAIN+LAUNCHER+DEFAULT manda sobre
// LEANBACK_LAUNCHER y sobre un alias, incluso cuando ambos van antes en el manifiesto.
func TestLanzadoraPriorizaActividadConDefault(t *testing.T) {
	ctx := contextoDeLanzadoraDePrueba()
	leanback := elementoDePrueba("activity",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.LeanbackActivity"),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LEANBACK_LAUNCHER"}))
	alias := elementoDePrueba("activity-alias",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "org.fdroid.IconActivity"),
		atributoTextoDePrueba(ctx, uriDeRecursos, "targetActivity", "org.fdroid.LeanbackActivity"),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LAUNCHER"}))
	preferida := elementoDePrueba("activity",
		atributoTextoDePrueba(ctx, uriDeRecursos, "name", "com.otra.RealActivity"),
		filtroDePrueba(ctx, []string{"android.intent.action.MAIN"},
			[]string{"android.intent.category.LAUNCHER", "android.intent.category.DEFAULT"}))
	elegida, ok := actividadLanzadora(manifiestoCon(ctx, leanback, alias, preferida), ctx)
	if !ok || elegida != preferida {
		t.Fatalf("se eligió %v y tenía que ganar la actividad con MAIN+LAUNCHER+DEFAULT",
			elegida)
	}
}

// tablaDeLayoutsDePrueba tiene dos entradas de layout: v9, con ruta acortada por aapt2, y
// activity_termux, con la ruta que escribe el enlazador de Go.
func tablaDeLayoutsDePrueba() *tablaRecursos {
	return &tablaRecursos{
		idPaquete: 0x7f,
		global: tablaCadenas{cadenas: []string{"res/v9.xml", "res/layout/activity_termux.xml"}},
		claves:    []string{"v9", "activity_termux"},
		bloques: []bloqueTipo{{id: 0x0b, nombre: "layout", config: make([]byte, 8),
			entradas: []entradaRecurso{
				{clave: 0, valor: valor{tipo: tipoCadena, datos: 0}},
				{clave: 1, valor: valor{tipo: tipoCadena, datos: 1}},
			}}},
	}
}

// TestRutaDelLayoutSetContentViewAntesQueNombre falsifica la regla 4: el id inflado gana al nombre,
// el nombre de la actividad gana a `main` y `main` gana al primero por orden alfabético.
func TestRutaDelLayoutSetContentViewAntesQueNombre(t *testing.T) {
	datosZip := zipDeEntradas(t, map[string][]byte{
		"res/layout/A1.xml":              []byte("no"),
		"res/layout/main.xml":            []byte("no"),
		"res/layout/activity_termux.xml": []byte("no"),
	})
	zr, err := zip.NewReader(bytes.NewReader(datosZip), int64(len(datosZip)))
	if err != nil {
		t.Fatalf("el zip: %v", err)
	}
	tabla := tablaDeLayoutsDePrueba()
	const actividad = "Lcom/termux/app/TermuxActivity;"

	if ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, "", 0, actividad); err != nil || ruta != "res/layout/activity_termux.xml" {
		t.Fatalf("por nombre se eligió %q (err %v) y se esperaba activity_termux.xml", ruta, err)
	}
	if ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, "", 0x7f0b0000, actividad); err != nil || ruta != "res/v9.xml" {
		t.Fatalf("con setContentView se eligió %q (err %v) y se esperaba res/v9.xml", ruta, err)
	}
	if ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, "main", 0x7f0b0000, actividad); err != nil || ruta != "res/layout/main.xml" {
		t.Fatalf("con --layout se eligió %q (err %v) y se esperaba main.xml", ruta, err)
	}
}

// TestSinLayoutXmlNoInventaPantalla falsifica la regla 5 de punta a punta: un apk con un aliased
// MAIN+LAUNCHER y un único `res/A1.xml` no puede caer a ese fichero; publica el centinela y el marco
// con la etiqueta de la app.
func TestSinLayoutXmlNoInventaPantalla(t *testing.T) {
	ctx := contextoDeLanzadoraDePrueba()
	manifiesto, err := escribeAXML(documentoAXML{
		ns: []declaracionNS{{prefijo: "android", uri: uriAndroid}},
		raiz: &elementoEscribir{nombre: "manifest", attrs: []atributoEscribir{
			{nombre: "package", tipo: tipoCadena, texto: "org.fdroid.fdroid"},
		}, hijos: []*elementoEscribir{{
			nombre: "application", attrs: []atributoEscribir{
				{ns: "android", nombre: "label", id: 0x01010001, tipo: tipoCadena, texto: "F-Droid"},
			},
			hijos: []*elementoEscribir{
				{nombre: "activity", attrs: []atributoEscribir{
					{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "org.fdroid.MainActivity"},
				}, hijos: []*elementoEscribir{{
					nombre: "intent-filter",
					hijos: []*elementoEscribir{
						{nombre: "action", attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.action.MAIN"},
						}},
						{nombre: "category", attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.category.LEANBACK_LAUNCHER"},
						}},
					},
				}}},
				{nombre: "activity-alias", attrs: []atributoEscribir{
					{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "org.fdroid.IconActivity"},
					{ns: "android", nombre: "targetActivity", id: 0x01010099, tipo: tipoCadena, texto: "org.fdroid.MainActivity"},
				}, hijos: []*elementoEscribir{{
					nombre: "intent-filter",
					hijos: []*elementoEscribir{
						{nombre: "action", attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.action.MAIN"},
						}},
						{nombre: "category", attrs: []atributoEscribir{
							{ns: "android", nombre: "name", id: 0x01010003, tipo: tipoCadena, texto: "android.intent.category.LAUNCHER"},
						}},
					},
				}}},
			},
		}},
	}})
	if err != nil {
		t.Fatalf("el manifiesto: %v", err)
	}
	axml, err := leeAXML(manifiesto)
	if err != nil {
		t.Fatalf("leer el manifiesto: %v", err)
	}
	ctx.axml = axml
	arbol, err := ctx.arbol()
	if err != nil {
		t.Fatalf("el árbol del manifiesto: %v", err)
	}
	elegida, ok := actividadLanzadora(arbol, ctx)
	if !ok || ctx.textoDeAtributo(elegida, uriDeRecursos, "name") != "org.fdroid.MainActivity" {
		t.Fatalf("el alias no llevó a MainActivity: %v", elegida)
	}
	inventada, err := escribeAXML(documentoAXML{raiz: &elementoEscribir{nombre: "FrameLayout"}})
	if err != nil {
		t.Fatalf("el layout estoy: %v", err)
	}
	datos := zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"res/A1.xml":          inventada,
	})
	m, err := modeloVistaDeAPKCon(datos, "", false)
	if err != nil {
		t.Fatalf("el modelo sin layout: %v", err)
	}
	if m.Layout != "" {
		t.Errorf("se inventó el layout %q", m.Layout)
	}
	if strings.Contains(m.Layout, "A1") {
		t.Errorf("el modelo cayó al primer XML: %q", m.Layout)
	}
	if m.Actividad != "org.fdroid.MainActivity" {
		t.Errorf("la actividad es %q y se esperaba la del targetActivity del alias", m.Actividad)
	}
	if m.Raiz == nil || m.Raiz.Tipo != "FrameLayout" || m.Raiz.Texto != "F-Droid" {
		t.Errorf("el marco raíz es %+v y tenía que llevar la etiqueta F-Droid", m.Raiz)
	}
	if m.Ejecucion == nil || len(m.Ejecucion.Avisos) != 1 ||
		!strings.HasPrefix(m.Ejecucion.Avisos[0], avisoSinLayout) {
		t.Errorf("los avisos son %v y tenían que traer el centinela", m.Ejecucion)
	}
}

// TestRutaDelLayoutDelExploradorSimple falsifica el mecanismo del cuarto APK medido: la tabla dice
// `layout/activity_main` → `res/R5.xml` y el nombre de su MainActivity es `activity_main`. Que el id
// de `setContentView` lleve al mismo sitio se comprueba aparte, porque las dos vías pueden coincidir.
func TestRutaDelLayoutDelExploradorSimple(t *testing.T) {
	tabla := &tablaRecursos{
		idPaquete: 0x7f,
		global:    tablaCadenas{cadenas: []string{"res/R5.xml"}},
		claves:    []string{"activity_main"},
		bloques: []bloqueTipo{{id: 0x04, nombre: "layout", config: make([]byte, 8),
			entradas: []entradaRecurso{{clave: 0, valor: valor{tipo: tipoCadena, datos: 0}}}}},
	}
	datosZip := zipDeEntradas(t, map[string][]byte{"res/A1.xml": []byte("no")})
	zr, err := zip.NewReader(bytes.NewReader(datosZip), int64(len(datosZip)))
	if err != nil {
		t.Fatalf("el zip: %v", err)
	}
	const actividad = "Lcom/martinmimigames/simplefileexplorer/MainActivity;"
	if ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, "", 0, actividad); err != nil || ruta != "res/R5.xml" {
		t.Errorf("por nombre del actividad se eligió %q (err %v) y se esperaba res/R5.xml", ruta, err)
	}
	if ruta, err := rutaDelLayoutDeLaActividad(tabla, zr, "", 0x7f040000, ""); err != nil || ruta != "res/R5.xml" {
		t.Errorf("por id de setContentView se eligió %q (err %v) y se esperaba res/R5.xml", ruta, err)
	}
}
