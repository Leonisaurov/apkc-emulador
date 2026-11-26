package main

// Los stubs de framework que destapó la APK de ApkShare: Looper, Context, PackageManager, el builder
// de AlertDialog y el ExecutorService. Antes tenían voz en el registro de «no está en el dex ni en
// los stubs» y devolvían el cero; encadenarlos terminaba en «se llamó a
// java.util.concurrent.ExecutorService.execute sobre un objeto nulo», que es lo que estas pruebas
// falsifican con el interruptor apagado.
//
// Todas las pruebas son directas sobre `intentaStub`: no necesitan dex, jar ni compilación, y por eso
// corren en el despacho puntual sin calentar el portátil.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maquinaDeStubs es una máquina mínima con el mapa de avisos puesto: `avisa` lo necesita.
func maquinaDeStubs() *maquinaDalvik {
	return &maquinaDalvik{vistos: map[string]bool{}}
}

// objDe es un objeto del framework con su clase y su mapa de campos listo.
func objDe(clase string) *objetoDalvik {
	return &objetoDalvik{clase: clase, campos: map[string]any{}}
}

// TestStubsDelFrameworkDeApkShare comprueba, uno a uno, que cada método de la cadena devuelve un
// objeto no nulo (o el propio builder) y que no cae en el aviso de stub ausente.
func TestStubsDelFrameworkDeApkShare(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lshare/apps/util/MainActivity;")
	m.actividad = actividad

	casos := []struct {
		nombre     string
		candidatos []string
		proto      string
		vals       []any
		receptor   any
		clase      string
	}{
		{"getMainLooper", []string{"Landroid/os/Looper;"}, "()Landroid/os/Looper;", nil, nil,
			"Landroid/os/Looper;"},
		{"getPackageManager", []string{"Lshare/apps/util/MainActivity;"},
			"()Landroid/content/pm/PackageManager;", nil, actividad,
			"Landroid/content/pm/PackageManager;"},
		{"getApplicationContext", []string{"Lshare/apps/util/MainActivity;"},
			"()Landroid/content/Context;", nil, actividad, "Landroid/app/Application;"},
		{"newSingleThreadExecutor", []string{"Ljava/util/concurrent/Executors;"},
			"()Ljava/util/concurrent/ExecutorService;", nil, nil,
			"Ljava/util/concurrent/ExecutorService;"},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor)
		if err != nil {
			t.Fatalf("%s devolvió error: %v", c.nombre, err)
		}
		if !ok {
			t.Fatalf("%s no lo atiende ningún stub", c.nombre)
		}
		o, esObjeto := v.(*objetoDalvik)
		if !esObjeto || o == nil {
			t.Errorf("%s devolvió %T y se esperaba un objeto no nulo", c.nombre, v)
			continue
		}
		if o.clase != c.clase {
			t.Errorf("%s devolvió %s y se esperaba %s", c.nombre, o.clase, c.clase)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna llamada cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestElContextoDevueltoEncadena: `getApplicationContext()` tiene que devolver un contexto sobre el
// que `getPackageManager` también responda; si el Application no se marcara, la app recibiría el
// cero en la segunda llamada.
func TestElContextoDevueltoEncadena(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lshare/apps/util/MainActivity;")
	m.actividad = actividad
	ctx, ok, err := m.intentaStub([]string{"Lshare/apps/util/MainActivity;"},
		"getApplicationContext", "()Landroid/content/Context;", nil, actividad)
	if err != nil || !ok {
		t.Fatalf("getApplicationContext: ok=%v err=%v", ok, err)
	}
	// el contexto de la app no es la actividad: en el framework es un Application propio, y el
	// intérprete no debe devolver el mismo objeto que recibió
	if ctx == nil {
		t.Fatal("getApplicationContext devolvió el cero")
	}
	if ctx == actividad {
		t.Error("getApplicationContext devolvió la propia Activity en vez de un Application distinto")
	}
	pm, ok, err := m.intentaStub([]string{"Lshare/apps/util/MainActivity;"},
		"getPackageManager", "()Landroid/content/pm/PackageManager;", nil, ctx)
	if err != nil || !ok {
		t.Fatalf("getPackageManager sobre el Application: ok=%v err=%v", ok, err)
	}
	if pm == nil {
		t.Error("getPackageManager sobre el Application devolvió el cero")
	}
	avisos := strings.Join(m.avisos, "\n")
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("el encadenamiento cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestElBuilderEncadenaComoEnElFramework: `setView` y `setCancelable` tienen que devolver el MISMO
// builder para que `create()` y `show()` sigan la cadena, y `hide()` quita el diálogo de la capa.
func TestElBuilderEncadenaComoEnElFramework(t *testing.T) {
	m := maquinaDeStubs()
	builder := objDe("Landroid/app/AlertDialog$Builder;")
	vista := objDe("Landroid/view/View;")
	for _, c := range []struct {
		nombre string
		proto  string
		vals   []any
	}{
		{"setView", "(Landroid/view/View;)Landroid/app/AlertDialog$Builder;", []any{vista}},
		{"setCancelable", "(Z)Landroid/app/AlertDialog$Builder;", []any{int32(0)}},
	} {
		v, ok, err := m.intentaStub([]string{"Landroid/app/AlertDialog$Builder;"},
			c.nombre, c.proto, c.vals, builder)
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", c.nombre, ok, err)
		}
		if v != builder {
			t.Errorf("%s no devolvió el builder: %v", c.nombre, v)
		}
	}
	d := creaDialogo(builder)
	m.apilaDialogo(d)
	if !m.dialogoAbierto(d) {
		t.Fatal("el diálogo creado no estaba abierto")
	}
	if _, ok, err := m.intentaStub([]string{"Landroid/app/Dialog;"}, "hide", "()V", nil, d); err != nil || !ok {
		t.Fatalf("hide: ok=%v err=%v", ok, err)
	}
	if m.dialogoAbierto(d) {
		t.Error("hide no quitó el diálogo de la capa")
	}
}

// TestElEjecutorEncolaElRunnable: `execute` tiene que encolar el Runnable en la cola lógica, como un
// `post`, en vez de devolver el cero y reventar al llamarlo.
func TestElEjecutorEncolaElRunnable(t *testing.T) {
	m := maquinaDeStubs()
	// el reloj y el mapa de clases son lo que necesita la cola para despachar el run() al vencer
	reloj := int64(0)
	m.reloj = &reloj
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	ejecutor := objDe("Ljava/util/concurrent/ExecutorService;")
	runnable := objDe("Lshare/apps/util/MainActivity$1;")
	v, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/ExecutorService;"},
		"execute", "(Ljava/lang/Runnable;)V", []any{runnable}, ejecutor)
	if err != nil || !ok {
		t.Fatalf("execute: ok=%v err=%v", ok, err)
	}
	if v != nil {
		t.Errorf("execute devolvió %v y es un método void", v)
	}
	if len(m.cola) != 1 {
		t.Fatalf("execute encoló %d tareas y se esperaba 1", len(m.cola))
	}
	if m.cola[0].runnable != runnable {
		t.Error("execute encoló un Runnable distinto del que recibió")
	}
	// execute solo encola: el Runnable no corre en la llamada, aunque la tarea venza en el instante
	// cero. Antes de mover el reloj no se ejecutó ninguna tarea ni se despachó ningún run().
	if m.tareas != 0 {
		t.Fatalf("execute ejecutó %d tareas de inmediato; el Runnable debe esperar al reloj", m.tareas)
	}
	if avisos := strings.Join(m.avisos, " "); strings.Contains(avisos, "no declara run()V") {
		t.Fatalf("execute despachó el run() antes de avanzar el reloj: %s", avisos)
	}
	if err := m.avanza(1); err != nil {
		t.Fatalf("avanzar el reloj falló: %v", err)
	}
	if !m.vence(m.ahora()) {
		t.Fatal("al avanzar el reloj no se ejecutó la tarea encolada")
	}
	if m.tareas != 1 {
		t.Errorf("tras avanzar se ejecutaron %d tareas y se esperaba 1", m.tareas)
	}
	if len(m.cola) != 0 {
		t.Errorf("tras avanzar quedan %d tareas en la cola", len(m.cola))
	}
	if avisos := strings.Join(m.avisos, " "); !strings.Contains(avisos, "no declara run()V") {
		t.Errorf("al vencer no se despachó el run() del Runnable: %s", avisos)
	}
}

// TestLaListaDePaquetesEsVaciaPeroNoNula: sin sistema de paquetes, `getInstalledPackages` devuelve
// una lista vacía y su `iterator()` un iterador sin elementos, para que el bucle de la app termine
// sin un receptor nulo.
func TestLaListaDePaquetesEsVaciaPeroNoNula(t *testing.T) {
	m := maquinaDeStubs()
	pm := objDe("Landroid/content/pm/PackageManager;")
	lista, ok, err := m.intentaStub([]string{"Landroid/content/pm/PackageManager;"},
		"getInstalledPackages", "(I)Ljava/util/List;", []any{int32(0)}, pm)
	if err != nil || !ok || lista == nil {
		t.Fatalf("getInstalledPackages: v=%v ok=%v err=%v", lista, ok, err)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/List;"}, "iterator", "()Ljava/util/Iterator;",
		nil, lista)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator: v=%v ok=%v err=%v", it, ok, err)
	}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok {
		t.Fatalf("hasNext: ok=%v err=%v", ok, err)
	}
	if v != int32(0) {
		t.Errorf("hasNext devolvió %v y la lista está vacía", v)
	}
}

// TestFalsificacionConLosStubsApagados reproduce el fallo original: sin los stubs,
// `newSingleThreadExecutor` no se resuelve y el intérprete entrega el cero (nil); la llamada
// siguiente a `execute` sobre ese cero no casa con ningún stub y es el «se llamó a
// java.util.concurrent.ExecutorService.execute sobre un objeto nulo» del registro de la APK.
func TestFalsificacionConLosStubsApagados(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	m := maquinaDeStubs()
	v, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/Executors;"},
		"newSingleThreadExecutor", "()Ljava/util/concurrent/ExecutorService;", nil, nil)
	if err != nil || ok {
		t.Fatalf("con los stubs apagados newSingleThreadExecutor se resolvió: v=%v ok=%v err=%v",
			v, ok, err)
	}
	ejecutor := valorCero("Ljava/util/concurrent/ExecutorService;")
	if ejecutor != nil {
		t.Fatalf("el cero del tipo de retorno es %v y se esperaba el receptor nulo", ejecutor)
	}
	v, ok, err = m.intentaStub([]string{"Ljava/util/concurrent/ExecutorService;"},
		"execute", "(Ljava/lang/Runnable;)V", []any{objDe("Ljava/lang/Runnable;")}, ejecutor)
	if err != nil || ok {
		t.Fatalf("execute sobre el cero se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
}

// esperaObjetoDeStub comprueba el tipo del objeto que devuelve un stub y lo devuelve para poder
// mirar su estado después.
func esperaObjetoDeStub(t *testing.T, v any, clase string) *objetoDalvik {
	t.Helper()
	o, ok := v.(*objetoDalvik)
	if !ok || o == nil {
		t.Fatalf("se esperaba un objeto %s y llegó %T (%v)", clase, v, v)
	}
	if o.clase != clase {
		t.Errorf("el objeto es %s y se esperaba %s", o.clase, clase)
	}
	return o
}

// esperaCadenaDeStub comprueba que el stub devolvió una cadena no nula con el texto esperado.
func esperaCadenaDeStub(t *testing.T, v any, esperado string) {
	t.Helper()
	s, ok := cadenaDe(v)
	if !ok {
		t.Fatalf("se esperaba una cadena y llegó %T (%v)", v, v)
	}
	if s != esperado {
		t.Errorf("la cadena es %q y se esperaba %q", s, esperado)
	}
}

// TestLosStubsDelCuartoApk falsifica uno a uno los doce métodos de la línea base §366, más los que
// el flujo real pidió después de que `ejecucion.error` desapareciera. Cada caso comprueba su
// semántica: los objetos son no nulos, las cadenas llevan el valor plausible y los setters guardan
// estado en su vista o en su diálogo sin tocar el layout de la pantalla.
func TestLosStubsDelCuartoApk(t *testing.T) {
	m := maquinaDeStubs()
	m.tabla = &tablaRecursos{nombrePaquete: "com.martinmimigames.simplefileexplorer"}
	actividad := objDe("Lcom/martinmimigames/simplefileexplorer/MainActivity;")
	m.actividad = actividad
	m.layoutID = 0x7f050000

	intencion := objDe("Landroid/content/Intent;")
	parametros := objDe("Landroid/widget/RelativeLayout$LayoutParams;")
	dialogo := objDe("Landroid/app/Dialog;")
	vista := objDe("Landroid/view/View;")
	vista.id = 0x7f030005
	contenedor := objDe("Landroid/widget/LinearLayout;")
	recursos := objDe("Landroid/content/res/Resources;")
	fichero := objDe("Ljava/io/File;")
	fichero.campos["file.ruta"] = "/sdcard/Download"

	casos := []struct {
		nombre     string
		candidatos []string
		proto      string
		vals       []any
		receptor   any
		comprueba  func(t *testing.T, v any)
	}{
		{"newCachedThreadPool", []string{"Ljava/util/concurrent/Executors;"},
			"()Ljava/util/concurrent/ExecutorService;", nil, nil,
			func(t *testing.T, v any) {
				esperaObjetoDeStub(t, v, "Ljava/util/concurrent/ExecutorService;")
			}},
		{"getClass", []string{"Ljava/lang/Object;"}, "()Ljava/lang/Class;", nil,
			cadenaObjeto("android.intent.action.MAIN"),
			func(t *testing.T, v any) {
				o := esperaObjetoDeStub(t, v, "Ljava/lang/Class;")
				if tipo := o.campos["Ljava/lang/Class;.$tipo"]; tipo != "Ljava/lang/String;" {
					t.Errorf("el Class lleva %v y se esperaba Ljava/lang/String;", tipo)
				}
			}},
		{"getPackageName", []string{"Lcom/martinmimigames/simplefileexplorer/MainActivity;"},
			"()Ljava/lang/String;", nil, actividad,
			func(t *testing.T, v any) {
				esperaCadenaDeStub(t, v, "com.martinmimigames.simplefileexplorer")
			}},
		{"setTheme", []string{"Landroid/content/Context;"}, "(I)V", []any{int32(1)}, actividad,
			func(t *testing.T, v any) {
				if v != nil {
					t.Errorf("setTheme devolvió %v y es void", v)
				}
			}},
		{"setCancelable", []string{"Landroid/app/Dialog;"}, "(Z)V", []any{int32(0)}, dialogo,
			func(t *testing.T, v any) {
				if v != nil {
					t.Errorf("setCancelable devolvió %v y es void", v)
				}
			}},
		{"setTitle", []string{"Landroid/app/Dialog;"}, "(Ljava/lang/CharSequence;)V",
			[]any{cadenaObjeto("Titulo")}, dialogo,
			func(t *testing.T, v any) {
				if v != nil {
					t.Errorf("setTitle devolvió %v y es void", v)
				}
				if titulo := dialogo.campos[campoDialogoTitulo]; titulo != "Titulo" {
					t.Errorf("el título guardado es %v", titulo)
				}
			}},
		{"setLayoutParams", []string{"Landroid/view/View;"},
			"(Landroid/view/ViewGroup$LayoutParams;)V", []any{parametros}, vista,
			func(t *testing.T, v any) {
				if v != nil || vista.campos["vista.layoutParams"] != parametros {
					t.Errorf("setLayoutParams: v=%v campos=%v", v, vista.campos)
				}
			}},
		{"setId", []string{"Landroid/view/View;"}, "(I)V", []any{int32(0x7f030006)}, vista,
			func(t *testing.T, v any) {
				if v != nil || vista.id != 0x7f030006 {
					t.Errorf("setId: v=%v id=%#08x", v, vista.id)
				}
			}},
		{"addView", []string{"Landroid/view/ViewGroup;"}, "(Landroid/view/View;)V",
			[]any{vista}, contenedor,
			func(t *testing.T, v any) {
				if v != nil || contenedor.vistas[vista.id] != vista {
					t.Errorf("addView: v=%v hijos=%v", v, contenedor.vistas)
				}
			}},
		{"addRule", []string{"Landroid/widget/RelativeLayout$LayoutParams;"}, "(II)V",
			[]any{int32(3), int32(0x7f030005)}, parametros,
			func(t *testing.T, v any) {
				reglas, _ := parametros.campos["layoutParams.reglas"].([]string)
				if v != nil || len(reglas) != 1 {
					t.Errorf("addRule: v=%v reglas=%v", v, reglas)
				}
			}},
		{"setContentView", []string{"Landroid/app/Dialog;"}, "(Landroid/view/View;)V",
			[]any{vista}, dialogo,
			func(t *testing.T, v any) {
				if v != nil || dialogo.campos["dialogo.vista"] != vista {
					t.Errorf("setContentView del Dialog: v=%v campos=%v", v, dialogo.campos)
				}
				if m.layoutID != 0x7f050000 {
					t.Errorf("setContentView del Dialog cambió el layout a %#08x", m.layoutID)
				}
			}},
		{"getAction", []string{"Landroid/content/Intent;"}, "()Ljava/lang/String;", nil,
			intencion,
			func(t *testing.T, v any) {
				esperaCadenaDeStub(t, v, "android.intent.action.MAIN")
			}},
		// los que el cuarto APK pidió después de seguir por su camino real
		{"getSystem", []string{"Landroid/content/res/Resources;"},
			"()Landroid/content/res/Resources;", nil, nil,
			func(t *testing.T, v any) {
				esperaObjetoDeStub(t, v, "Landroid/content/res/Resources;")
			}},
		{"getDisplayMetrics", []string{"Landroid/content/res/Resources;"},
			"()Landroid/util/DisplayMetrics;", nil, recursos,
			func(t *testing.T, v any) {
				o := esperaObjetoDeStub(t, v, "Landroid/util/DisplayMetrics;")
				if ancho := o.campos["Landroid/util/DisplayMetrics;.widthPixels"]; ancho != int32(360) {
					t.Errorf("el ancho es %v y se esperaba 360", ancho)
				}
			}},
		{"getAbsoluteFile", []string{"Ljava/io/File;"}, "()Ljava/io/File;", nil, fichero,
			func(t *testing.T, v any) {
				o := esperaObjetoDeStub(t, v, "Ljava/io/File;")
				if o.campos["file.ruta"] != "/sdcard/Download" {
					t.Errorf("getAbsoluteFile perdió la ruta: %v", o.campos)
				}
			}},
		{"getPath", []string{"Ljava/io/File;"}, "()Ljava/lang/String;", nil, fichero,
			func(t *testing.T, v any) {
				esperaCadenaDeStub(t, v, "/sdcard/Download")
			}},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor)
		if err != nil || !ok {
			t.Errorf("%s: v=%v ok=%v err=%v", c.nombre, v, ok, err)
			continue
		}
		c.comprueba(t, v)
	}

	// un Intent construido con `new Intent(String)` conserva su acción, en vez de recibir siempre
	// la acción genérica de la actividad lanzadora
	intencion.campos["intent.accion"] = "android.intent.action.VIEW"
	v, ok, err := m.intentaStub([]string{"Landroid/content/Intent;"}, "getAction",
		"()Ljava/lang/String;", nil, intencion)
	if err != nil || !ok {
		t.Fatalf("getAction con acción explícita: v=%v ok=%v err=%v", v, ok, err)
	}
	esperaCadenaDeStub(t, v, "android.intent.action.VIEW")

	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("algún stub del cuarto APK cayó en el aviso de familia: %s", avisos)
	}
}

// TestElGuardiánDeNuloSigueCortandoAntesDelStub comprueba que el stub de getClass NO relaja el
// guardián de `invoca`: con receptor nulo el error sigue siendo el de siempre.
func TestElGuardiánDeNuloSigueCortandoAntesDelStub(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/lang/Object;", name: "getClass",
		proto: dexProto{returnType: "Ljava/lang/Class;"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "")
	if err == nil {
		t.Fatal("getClass con receptor nulo no devolvió error")
	}
	if !strings.Contains(err.Error(), "se llamó a java.lang.Object.getClass sobre un objeto nulo") {
		t.Errorf("el error fue %q y se esperaba el del guardián de receptor nulo", err)
	}
}

// TestLaCadenaRealGetIntentGetActionGetClass recorre los tres invoke del dex del cuarto APK
// (dexdump 005694..0056a4): el nulo lo producía `getAction()`, no `getClass()`. Los tres pasos
// tienen que terminar sin error y sin avisos de la familia.
func TestLaCadenaRealGetIntentGetActionGetClass(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	actividad := objDe("Lcom/martinmimigames/simplefileexplorer/MainActivity;")
	m.actividad = actividad
	d := &dexFile{methods: []dexMethod{
		{owner: "Landroid/app/Activity;", name: "getIntent",
			proto: dexProto{returnType: "Landroid/content/Intent;"}},
		{owner: "Landroid/content/Intent;", name: "getAction",
			proto: dexProto{returnType: "Ljava/lang/String;"}},
		{owner: "Ljava/lang/Object;", name: "getClass",
			proto: dexProto{returnType: "Ljava/lang/Class;"}},
	}}

	intencion, err := m.invoca(0x6e, d, 0, []int{0}, []any{actividad}, "")
	if err != nil {
		t.Fatalf("getIntent falló: %v", err)
	}
	accion, err := m.invoca(0x6e, d, 1, []int{0}, []any{intencion}, "")
	if err != nil {
		t.Fatalf("getAction falló: %v", err)
	}
	esperaCadenaDeStub(t, accion, "android.intent.action.MAIN")
	clase, err := m.invoca(0x6e, d, 2, []int{0}, []any{accion}, "")
	if err != nil {
		t.Fatalf("getClass falló: %v", err)
	}
	esperaObjetoDeStub(t, clase, "Ljava/lang/Class;")
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("la cadena real dejó avisos de familia: %s", avisos)
	}
}

// TestLaEvidenciaDelCuartoApk lee el JSON medido con el binario post-D y exige las dos condiciones
// que el criterio falsable no puede ver con solo mirar `m.layoutID`: el layout y los nodos siguen
// siendo los de la línea base §366, y la familia «llamada que no se sabe resolver» está a cero con
// `ejecucion.error` ausente. El centinela `sin-layout-xml:` de Fase C se excluye del recuento.
func TestLaEvidenciaDelCuartoApk(t *testing.T) {
	casa, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("no se pudo resolver el directorio de casa: %v", err)
	}
	ruta := filepath.Join(casa, ".cache", "apkc-web", "fase-d-evidencia", "preview-post.json")
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	var modelo struct {
		Layout    string `json:"layout"`
		Nodos     int    `json:"nodos"`
		Ejecucion struct {
			Avisos []string `json:"avisos"`
			Error  string   `json:"error"`
		} `json:"ejecucion"`
	}
	if err := json.Unmarshal(datos, &modelo); err != nil {
		t.Fatalf("%s no es JSON válido: %v", ruta, err)
	}
	if modelo.Layout != "res/R5.xml" {
		t.Errorf("el layout es %q y se esperaba res/R5.xml", modelo.Layout)
	}
	if modelo.Nodos != 30 {
		t.Errorf("los nodos son %d y se esperaban 30", modelo.Nodos)
	}
	familia := 0
	for _, aviso := range modelo.Ejecucion.Avisos {
		if strings.Contains(aviso, "no está en el dex ni en los stubs") &&
			!strings.Contains(aviso, "sin-layout-xml:") {
			familia++
		}
	}
	if familia != 0 {
		t.Errorf("queda %d aviso(s) de la familia: %v", familia, modelo.Ejecucion.Avisos)
	}
	if modelo.Ejecucion.Error != "" {
		t.Errorf("ejecucion.error no está ausente: %q", modelo.Ejecucion.Error)
	}
}

// --- familia 5: red registrada, sin una sola petición ----------------------------------------

// transportProhibido es la falsación de red: si algo en el intérprete intentara una petición con el
// cliente por omisión de Go, el bandera se enciende y la petición falla antes de salir del proceso.
type transportProhibido struct {
	usado *bool
}

func (tr transportProhibido) RoundTrip(*http.Request) (*http.Response, error) {
	*tr.usado = true
	return nil, errors.New("la vista previa no debe abrir peticiones de red")
}

// casoRedSinPeticion describe una llamada de la familia 5 y lo que debe devolver sin tocar red.
type casoRedSinPeticion struct {
	nombre     string
	candidatos []string
	proto      string
	vals       []any
	receptor   any
	comprueba  func(t *testing.T, v any)
}

// casosRedSinPeticion son los nueve métodos encargados: cinco de HttpURLConnection y cuatro de
// okhttp3. Comprueban el valor vacío y no nulo que la app necesita para seguir sin datos inventados.
func casosRedSinPeticion() []casoRedSinPeticion {
	conexion := objDe("Ljava/net/HttpURLConnection;")
	cliente := objDe("Lokhttp3/OkHttpClient;")
	llamada := objDe("Lokhttp3/Call;")
	respuesta := objDe("Lokhttp3/Response;")
	cuerpo := objDe("Lokhttp3/ResponseBody;")
	esperaNulo := func(t *testing.T, v any) {
		t.Helper()
		if v != nil {
			t.Errorf("el método devolvió %v y es void", v)
		}
	}
	return []casoRedSinPeticion{
		{"connect", []string{"Ljava/net/HttpURLConnection;"}, "()V", nil, conexion, esperaNulo},
		{"getResponseCode", []string{"Ljava/net/HttpURLConnection;"}, "()I", nil, conexion,
			func(t *testing.T, v any) {
				if v != int32(200) {
					t.Errorf("getResponseCode devolvió %v y se esperaba 200", v)
				}
			}},
		{"getHeaderField", []string{"Ljava/net/URLConnection;"},
			"(Ljava/lang/String;)Ljava/lang/String;", []any{cadenaObjeto("Content-Type")}, conexion,
			func(t *testing.T, v any) {
				esperaCadenaDeStub(t, v, "")
			}},
		{"getInputStream", []string{"Ljava/net/HttpURLConnection;"},
			"()Ljava/io/InputStream;", nil, conexion,
			func(t *testing.T, v any) {
				o := esperaObjetoDeStub(t, v, "Ljava/io/ByteArrayInputStream;")
				if cuerpo := o.campos["red.cuerpo"]; cuerpo != "" {
					t.Errorf("el cuerpo de la respuesta es %v y se esperaba vacío", cuerpo)
				}
				if o.lista == nil || len(o.lista) != 0 {
					t.Errorf("el cuerpo trae bytes %v y se esperaba vacío", o.lista)
				}
			}},
		{"disconnect", []string{"Ljava/net/URLConnection;"}, "()V", nil, conexion, esperaNulo},
		{"newCall", []string{"Lokhttp3/OkHttpClient;"},
			"(Lokhttp3/Request;)Lokhttp3/Call;", []any{objDe("Lokhttp3/Request;")}, cliente,
			func(t *testing.T, v any) {
				esperaObjetoDeStub(t, v, "Lokhttp3/Call;")
			}},
		{"execute", []string{"Lokhttp3/Call;"}, "()Lokhttp3/Response;", nil, llamada,
			func(t *testing.T, v any) {
				esperaObjetoDeStub(t, v, "Lokhttp3/Response;")
			}},
		{"body", []string{"Lokhttp3/Response;"}, "()Lokhttp3/ResponseBody;", nil, respuesta,
			func(t *testing.T, v any) {
				esperaObjetoDeStub(t, v, "Lokhttp3/ResponseBody;")
			}},
		{"string", []string{"Lokhttp3/ResponseBody;"}, "()Ljava/lang/String;", nil, cuerpo,
			func(t *testing.T, v any) {
				esperaCadenaDeStub(t, v, "")
			}},
	}
}

// TestRedSinPeticionDevuelveRespuestaVaciaYRegistraAviso falsifica el contrato de la familia 5: los
// nueve métodos se resuelven, su valor es vacío y determinista y cada uno deja su aviso
// `red-sin-peticion: <metodo>`. Ninguno cae en el aviso genérico de «no está en el dex ni en los
// stubs», que es el que delataba antes una llamada sin modelo.
func TestRedSinPeticionDevuelveRespuestaVaciaYRegistraAviso(t *testing.T) {
	m := maquinaDeStubs()
	for _, c := range casosRedSinPeticion() {
		v, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor)
		if err != nil || !ok {
			t.Errorf("%s: v=%v ok=%v err=%v", c.nombre, v, ok, err)
			continue
		}
		c.comprueba(t, v)
		if aviso := "red-sin-peticion: " + c.nombre; !contieneTexto(m.avisos, aviso) {
			t.Errorf("%s no registró %q; avisos=%v", c.nombre, aviso, m.avisos)
		}
	}
	for _, aviso := range m.avisos {
		if strings.Contains(aviso, "no está en el dex ni en los stubs") {
			t.Errorf("alguna llamada de red cayó en el aviso genérico: %s", aviso)
		}
	}
}

// TestRedSinPeticionNoAbreTransporte inyecta un transporte de Go que falla y enciende un bandera si
// se usa, y lo deja instalado mientras corren los nueve stubs. Si el intérprete intentase una sola
// petición con el cliente por omisión, la bandera lo diría; no es una suposición sobre el código.
func TestRedSinPeticionNoAbreTransporte(t *testing.T) {
	usado := false
	anterior := http.DefaultTransport
	http.DefaultTransport = transportProhibido{usado: &usado}
	defer func() { http.DefaultTransport = anterior }()

	m := maquinaDeStubs()
	for _, c := range casosRedSinPeticion() {
		if _, ok, err := m.intentaStub(c.candidatos, c.nombre, c.proto, c.vals, c.receptor); err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", c.nombre, ok, err)
		}
	}
	if usado {
		t.Fatal("se intentó una petición de red: el transporte prohibido fue invocado")
	}
	if len(m.avisos) == 0 {
		t.Fatal("las llamadas de red no dejaron ningún aviso registrado")
	}
}

// TestRedSinPeticionSinImportesDeRed cierra la otra vía: el intérprete no importa paquetes de red ni
// el puente de JavaScript, así que no hay socket, cliente HTTP ni llamada al navegador escondida en
// este fichero. Se analiza la copia materializada en apkc/, que es la que compila el binario.
func TestRedSinPeticionSinImportesDeRed(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "previewinterprete.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("no se pudo analizar previewinterprete.go: %v", err)
	}
	prohibidos := map[string]bool{"net": true, "net/http": true, "syscall/js": true}
	for _, imp := range f.Imports {
		ruta := strings.Trim(imp.Path.Value, "\"")
		if prohibidos[ruta] {
			t.Errorf("previewinterprete.go importa %s: la familia 5 no debe tener red", ruta)
		}
	}
}

// TestLifecycleDelCompanionAlObservador recorre los tres stubs que destaparon las dos APK reales:
// el Companion crea un controlador no nulo, Lifecycling elige el constructor por reflexión y el
// nombre canónico de un Class sin descriptor cae a su propia clase. Antes, el cero del primero
// llegaba como observador nulo a `LifecycleRegistry.addObserver` y reventaba en el `getClass()`
// de `Lifecycling.getObserverConstructorType`.
func TestLifecycleDelCompanionAlObservador(t *testing.T) {
	m := maquinaDeStubs()
	owner := objDe("Lorg/schabi/newpipe/MainActivity;")
	companion := objDe("Landroidx/savedstate/SavedStateRegistryController$Companion;")

	controlador, ok, err := m.intentaStub(
		[]string{"Landroidx/savedstate/SavedStateRegistryController$Companion;"},
		"create",
		"(Landroidx/savedstate/SavedStateRegistryOwner;)Landroidx/savedstate/SavedStateRegistryController;",
		[]any{owner}, companion)
	if err != nil || !ok {
		t.Fatalf("create del Companion: ok=%v err=%v", ok, err)
	}
	c := esperaObjetoDeStub(t, controlador,
		"Landroidx/savedstate/SavedStateRegistryController;")
	if c.campos["savedstate.owner"] != owner {
		t.Error("el controlador no guardó el owner que recibió en create")
	}
	// Kotlin emite el `create` del Companion como estático: sin receptor también tiene que casa.
	estatico, ok, err := m.intentaStub(
		[]string{"Landroidx/savedstate/SavedStateRegistryController$Companion;"},
		"create",
		"(Landroidx/savedstate/SavedStateRegistryOwner;)Landroidx/savedstate/SavedStateRegistryController;",
		[]any{owner}, nil)
	if err != nil || !ok {
		t.Fatalf("create estático del Companion: ok=%v err=%v", ok, err)
	}
	esperaObjetoDeStub(t, estatico, "Landroidx/savedstate/SavedStateRegistryController;")

	tipo, ok, err := m.intentaStub([]string{"Landroidx/lifecycle/Lifecycling;"},
		"getObserverConstructorType", "(Ljava/lang/Object;)I", []any{owner}, nil)
	if err != nil || !ok {
		t.Fatalf("getObserverConstructorType: ok=%v err=%v", ok, err)
	}
	if tipo != int32(1) {
		t.Errorf("getObserverConstructorType devolvió %v y se esperaba 1 (reflexión)", tipo)
	}

	// un Class sin el campo `$tipo` no debe dejar una cadena vacía: eso era lo que hacía caer el
	// `HashMap.get` siguiente en NewPipe.
	clase := objDe("Ljava/lang/Class;")
	nombre, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "getCanonicalName",
		"()Ljava/lang/String;", nil, clase)
	if err != nil || !ok {
		t.Fatalf("getCanonicalName: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, nombre, "java.lang.Class")

	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("la cadena de lifecycle cayó en el aviso de stub ausente: %s", avisos)
	}
}

// maquinaConTiposDeForName arma una máquina con la tabla de tipos que trae Libre Memory Game:
// `GodotApp` y `GeneratedAdapter` están escritos en el dex, y el adaptador `_LifecycleAdapter`
// que Lifecycling busca por nombre no está. Es lo que distingue un Class legítimo de un nombre
// que el APK no contiene.
func maquinaConTiposDeForName() *maquinaDalvik {
	d := &dexFile{types: []string{
		"",
		"Lcom/godot/game/GodotApp;",
		"Landroidx/lifecycle/GeneratedAdapter;",
		"Ljava/lang/String;",
	}}
	return nuevaMaquina(&programaDalvik{dex: d, clases: map[string]*claseDalvik{
		"Lcom/godot/game/GodotApp;": {nombre: "Lcom/godot/game/GodotApp;"},
	}}, nil)
}

// TestForNameConNombreDelDexDevuelveUnaClaseNoNula cierra la mitad fail-open: los nombres que sí
// están en la tabla de tipos se resuelven al Class sintético que ya usa `getClass`, con su
// descriptor, sin aviso de excepción y sin devolver nunca el cero.
func TestForNameConNombreDelDexDevuelveUnaClaseNoNula(t *testing.T) {
	m := maquinaConTiposDeForName()
	for _, nombre := range []string{"com.godot.game.GodotApp", "androidx.lifecycle.GeneratedAdapter"} {
		t.Run(nombre, func(t *testing.T) {
			m.avisos = nil
			m.vistos = map[string]bool{}
			v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "forName",
				"(Ljava/lang/String;)Ljava/lang/Class;", []any{nombre}, nil)
			if err != nil || !ok {
				t.Fatalf("forName(%s): ok=%v err=%v", nombre, ok, err)
			}
			clase := esperaObjetoDeStub(t, v, "Ljava/lang/Class;")
			esperado := "L" + strings.ReplaceAll(nombre, ".", "/") + ";"
			if got := claseDeLiteral(clase); got != esperado {
				t.Errorf("forName(%s) guardó el descriptor %q y tenía que guardar %q", nombre, got, esperado)
			}
			if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "ClassNotFoundException") {
				t.Errorf("forName(%s) lanzó una excepción para un nombre que sí está: %s", nombre, avisos)
			}
		})
	}
}

// TestForNameConNombreAusenteLanzaClassNotFoundException cierra la mitad fail-closed: un nombre
// que no está en el dex —en especial el `*_LifecycleAdapter` que buscaba Lifecycling— no se
// fabrica. Se lanza la excepción que el propio dex catcha y se registra el aviso, en vez de
// devolver el nulo que acababa en el cast a `Class<out GeneratedAdapter>`.
func TestForNameConNombreAusenteLanzaClassNotFoundException(t *testing.T) {
	m := maquinaConTiposDeForName()
	ausentes := []string{
		"com.godot.game.GodotApp_LifecycleAdapter",
		"androidx.lifecycle.GeneratedAdapter_LifecycleAdapter",
		"no.esta.EnElDex",
		"",
	}
	for _, nombre := range ausentes {
		t.Run(nombre, func(t *testing.T) {
			m.avisos = nil
			m.vistos = map[string]bool{}
			v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "forName",
				"(Ljava/lang/String;)Ljava/lang/Class;", []any{nombre}, nil)
			if !ok {
				t.Fatalf("forName(%q) tiene que resolverse dentro del stub", nombre)
			}
			if v != nil {
				t.Errorf("forName(%q) devolvió %v junto a la excepción", nombre, v)
			}
			ex := excepcionDe(err)
			if ex == nil || ex.tipo != "Ljava/lang/ClassNotFoundException;" {
				t.Fatalf("forName(%q) dio %v y tenía que lanzar ClassNotFoundException", nombre, err)
			}
			if !m.tipoAcepta("Ljava/lang/ClassNotFoundException;", ex.tipo) {
				t.Errorf("el catch del dex no aceptaría la excepción de forName(%q)", nombre)
			}
			if len(m.avisos) != 1 || !strings.Contains(m.avisos[0], "ClassNotFoundException") {
				t.Errorf("el aviso de forName(%q) es %v y tenía que registrar la excepción", nombre, m.avisos)
			}
		})
	}
}

// TestForNameNoAbreLaReflexionRestante fija que el stub nuevo no abre la puerta a fabricar el
// adaptador: `newInstance` sigue rechazándose por nombre, y `getDeclaredConstructor`,
// `isAccessible`, `setAccessible` y los `Field`/`Method` siguen sin tener stub (el aviso lo pone
// `invoca` al no resolverlos). Ninguno devuelve un Class ni una excepción nueva.
func TestForNameNoAbreLaReflexionRestante(t *testing.T) {
	m := maquinaConTiposDeForName()
	for _, nombre := range []string{
		"getMethod", "getDeclaredMethod", "newInstance", "invoke",
		"getDeclaredConstructor", "isAccessible", "setAccessible", "getField", "getDeclaredField",
	} {
		t.Run(nombre, func(t *testing.T) {
			m.avisos = nil
			m.vistos = map[string]bool{}
			v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;", "Ljava/lang/reflect/Constructor;"},
				nombre, "()Ljava/lang/Object;", nil, nil)
			if err != nil {
				t.Fatalf("%s lanzó %v y tenía que rechazarse con aviso", nombre, err)
			}
			if ok && v != nil {
				t.Errorf("%s devolvió %v y no tiene stub", nombre, v)
			}
			if ok && len(m.avisos) == 0 {
				t.Errorf("%s se aceptó sin aviso", nombre)
			}
		})
	}
}

// TestMetodosVaciosDelObservadorSinClassDef fija el stub de `Class.getDeclaredMethods`: para una
// clase sintética sin `class_def` en el dex devuelve un arreglo vacío tipado, con aviso explícito de
// aproximación y sin fabricar ningún `Method`. Es lo que deja a `ClassesInfoCache.hasLifecycleMethods`
// escribir `false` sin llegar al `array-length` de un registro nulo.
func TestMetodosVaciosDelObservadorSinClassDef(t *testing.T) {
	m := maquinaConTiposDeForName()
	m.avisos = nil
	m.vistos = map[string]bool{}
	sintetica := &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
		"Ljava/lang/Class;.$tipo": "Landroidx/activity46ComponentActivity$2_LifecycleAdapter;",
	}}
	v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "getDeclaredMethods",
		"()[Ljava/lang/reflect/Method;", nil, sintetica)
	if err != nil || !ok {
		t.Fatalf("getDeclaredMethods sobre una clase sintética: ok=%v err=%v", ok, err)
	}
	arr, ok := v.(*arregloDalvik)
	if !ok || arr == nil {
		t.Fatalf("se esperaba un arreglo y llegó %T (%v)", v, v)
	}
	if arr.tipo != "[Ljava/lang/reflect/Method;" {
		t.Errorf("el arreglo es %q y tenía que ser [Ljava/lang/reflect/Method;", arr.tipo)
	}
	if len(arr.datos) != 0 {
		t.Errorf("el arreglo trae %d elementos y tenía que estar vacío (no se fabrica Method)", len(arr.datos))
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "getDeclaredMethods") || !strings.Contains(avisos, "aproximación") {
		t.Errorf("el aviso %q no nombra el stub y la aproximación", avisos)
	}
}

// TestMetodosDeClaseDelDexNoSeStubbean comprueba la frontera: una clase con `class_def` en el dex
// NO recibe el arreglo vacío, para no ocultar los callbacks reales que la app haya escrito.
func TestMetodosDeClaseDelDexNoSeStubbean(t *testing.T) {
	m := maquinaConTiposDeForName()
	m.avisos = nil
	m.vistos = map[string]bool{}
	conDef := &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
		"Ljava/lang/Class;.$tipo": "Lcom/godot/game/GodotApp;",
	}}
	v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "getDeclaredMethods",
		"()[Ljava/lang/reflect/Method;", nil, conDef)
	if err != nil {
		t.Fatalf("getDeclaredMethods sobre una clase del dex lanzó %v", err)
	}
	if ok && v != nil {
		t.Errorf("la clase del dex recibió el arreglo vacío: %v", v)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "aproximación") {
		t.Errorf("la clase del dex se stubbeó con aviso de aproximación: %s", avisos)
	}
}

// TestMetodosDelObservadorDelFrameworkConClassDev fija la otra mitad de la frontera: el observador
// anónimo de AndroidX sí tiene `class_def` en el dex, pero su notificación ya la modela
// `getObserverConstructorType`. Se devuelve el arreglo vacío sin fabricar `Method` ni adaptador.
func TestMetodosDelObservadorDelFrameworkConClassDev(t *testing.T) {
	m := maquinaConTiposDeForName()
	m.avisos = nil
	m.vistos = map[string]bool{}
	framework := &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
		"Ljava/lang/Class;.$tipo": "Landroidx/activity/ComponentActivity$2;",
	}}
	v, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "getDeclaredMethods",
		"()[Ljava/lang/reflect/Method;", nil, framework)
	if err != nil || !ok {
		t.Fatalf("getDeclaredMethods sobre el observador de AndroidX: ok=%v err=%v", ok, err)
	}
	arr, ok := v.(*arregloDalvik)
	if !ok || arr == nil {
		t.Fatalf("se esperaba un arreglo y llegó %T (%v)", v, v)
	}
	if arr.tipo != "[Ljava/lang/reflect/Method;" || len(arr.datos) != 0 {
		t.Errorf("el arreglo es %v con %d elementos y tenía que estar vacío y tipado", arr.tipo, len(arr.datos))
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "aproximación") {
		t.Errorf("el aviso %q no declara la aproximación", avisos)
	}
}

// TestLifecycleElPoolFijoDevuelveUnEjecutor añade `newFixedThreadPool` a la familia de ejecutores:
// recibe argumentos (a diferencia de los otros dos) y por eso la condición de aridad tuvo que
// ampliarse. Sin esto, NewPipe y Libre Memory Game seguían con un ExecutorService nulo.
func TestLifecycleElPoolFijoDevuelveUnEjecutor(t *testing.T) {
	m := maquinaDeStubs()
	fabrica := objDe("Ljava/util/concurrent/ThreadFactory;")
	for _, c := range []struct {
		proto string
		vals  []any
	}{
		{"(I)Ljava/util/concurrent/ExecutorService;", []any{int32(4)}},
		{"(ILjava/util/concurrent/ThreadFactory;)Ljava/util/concurrent/ExecutorService;",
			[]any{int32(4), fabrica}},
	} {
		v, ok, err := m.intentaStub([]string{"Ljava/util/concurrent/Executors;"},
			"newFixedThreadPool", c.proto, c.vals, nil)
		if err != nil || !ok {
			t.Fatalf("newFixedThreadPool%s: ok=%v err=%v", c.proto, ok, err)
		}
		esperaObjetoDeStub(t, v, "Ljava/util/concurrent/ExecutorService;")
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("newFixedThreadPool cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestInertesLoadLibraryNoCargaNada falsifica §4 R9: System.loadLibrary se acepta sin abrir un .so,
// sin ejecutar código nativo y sin dejar un objeto que pretenda ser la biblioteca cargada. El único
// efecto observable es el aviso de la familia.
func TestInertesLoadLibraryNoCargaNada(t *testing.T) {
	m := maquinaDeStubs()
	v, ok, err := m.intentaStub([]string{"Ljava/lang/System;"}, "loadLibrary",
		"(Ljava/lang/String;)V", []any{cadenaObjeto("godot_android")}, nil)
	if err != nil || !ok {
		t.Fatalf("loadLibrary: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != nil {
		t.Errorf("loadLibrary devolvió %v y es void", v)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "nativo-inerte: java.lang.System.loadLibrary") {
		t.Errorf("loadLibrary no registró su aviso: %q", m.avisos)
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("loadLibrary cayó en el aviso genérico: %q", m.avisos)
	}
	// la forma System.load es la misma familia y tampoco carga nada
	if v, ok, err := m.intentaStub([]string{"Ljava/lang/System;"}, "load",
		"(Ljava/lang/String;)V", []any{cadenaObjeto("/sdcard/libgodot_android.so")}, nil); err != nil ||
		!ok || v != nil {
		t.Errorf("System.load: v=%v ok=%v err=%v", v, ok, err)
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "nativo-inerte: java.lang.System.load") {
		t.Errorf("System.load no registró su aviso: %q", m.avisos)
	}
}

// TestInertesWebViewNoPideRed recorre la cadena WebView → WebSettings → WebChromeClient y comprueba
// que cada llamada entrega un objeto no nulo o void, con aviso de familia, y sin el aviso genérico.
// Ningún stub de esta cadena abre una conexión: la garantía de red se falsifica además en el test de
// importaciones de más abajo.
func TestInertesWebViewNoPideRed(t *testing.T) {
	m := maquinaDeStubs()
	// el `<init>` consulta la jerarquía del dex para decidir si el receptor es un Throwable: sin
	// programa cargado el mapa tiene que existir, aunque esté vacío
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	vista := objDe("Landroid/webkit/WebView;")
	if v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, "<init>",
		"(Landroid/content/Context;)V", []any{objDe("Landroid/app/Activity;")}, vista); err != nil ||
		!ok || v != nil {
		t.Fatalf("WebView.<init>: v=%v ok=%v err=%v", v, ok, err)
	}
	if vista.campos[markerInerteWeb] == nil {
		t.Fatal("el WebView no quedó marcado como inerte")
	}
	ajustes := esperaObjetoDeStub(t, mustStub(t, m, []string{"Landroid/webkit/WebView;"},
		"getSettings", "()Landroid/webkit/WebSettings;", nil, vista), "Landroid/webkit/WebSettings;")
	if ajustes.campos[markerInerteWeb] == nil {
		t.Error("los WebSettings devueltos no quedaron marcados como inertes")
	}
	if v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebSettings;"}, "setJavaScriptEnabled",
		"(Z)V", []any{int32(1)}, ajustes); err != nil || !ok || v != nil {
		t.Errorf("setJavaScriptEnabled: v=%v ok=%v err=%v", v, ok, err)
	}
	if v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, "loadUrl",
		"(Ljava/lang/String;)V", []any{cadenaObjeto("https://ejemplo.invalido")}, vista); err != nil ||
		!ok || v != nil {
		t.Errorf("loadUrl: v=%v ok=%v err=%v", v, ok, err)
	}
	cliente := objDe("Landroid/webkit/WebChromeClient;")
	if v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebChromeClient;"}, "<init>",
		"()V", nil, cliente); err != nil || !ok || v != nil {
		t.Fatalf("WebChromeClient.<init>: v=%v ok=%v err=%v", v, ok, err)
	}
	if v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebChromeClient;"}, "onProgressChanged",
		"(Landroid/webkit/WebView;I)V", []any{vista, int32(100)}, cliente); err != nil || !ok || v != nil {
		t.Errorf("onProgressChanged: v=%v ok=%v err=%v", v, ok, err)
	}
	avisos := strings.Join(m.avisos, "\n")
	for _, esperado := range []string{
		"webview-inerte: android.webkit.WebView.<init>",
		"webview-inerte: android.webkit.WebView.getSettings",
		"webview-inerte: android.webkit.WebSettings.setJavaScriptEnabled",
		"webview-inerte: android.webkit.WebView.loadUrl",
		"webview-inerte: android.webkit.WebChromeClient.onProgressChanged",
	} {
		if !strings.Contains(avisos, esperado) {
			t.Errorf("falta el aviso %q en %q", esperado, m.avisos)
		}
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("la cadena WebView cayó en el aviso genérico: %q", m.avisos)
	}
	// sin página cargada, getUrl() y getTitle() son texto vacío, nunca una URL inventada
	for _, nombre := range []string{"getUrl", "getTitle"} {
		v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, nombre,
			"()Ljava/lang/String;", nil, vista)
		if err != nil || !ok {
			t.Errorf("%s: ok=%v err=%v", nombre, ok, err)
			continue
		}
		esperaCadenaDeStub(t, v, "")
	}
}

// mustStub es el atajo de las pruebas de inertes cuando el valor devuelto tiene que ser un objeto.
func mustStub(t *testing.T, m *maquinaDalvik, candidatos []string, nombre, proto string,
	vals []any, receptor any) any {
	t.Helper()
	v, ok, err := m.intentaStub(candidatos, nombre, proto, vals, receptor)
	if err != nil || !ok {
		t.Fatalf("%s: v=%v ok=%v err=%v", nombre, v, ok, err)
	}
	return v
}

// TestInertesExoPlayerDevuelveObjetoInerte falsifica la familia de NewPipe: las fábricas de
// ExoPlayer/Media3 devuelven referencias no nulas, los primitivos devuelven su cero y los objetos
// encadenados siguen siendo inertes, sin aviso genérico.
func TestInertesExoPlayerDevuelveObjetoInerte(t *testing.T) {
	m := maquinaDeStubs()
	builder := esperaObjetoDeStub(t, mustStub(t, m,
		[]string{"Landroidx/media3/exoplayer/ExoPlayer$Builder;"}, "Builder",
		"(Landroid/content/Context;)Landroidx/media3/exoplayer/ExoPlayer$Builder;",
		[]any{objDe("Landroid/app/Activity;")}, nil), "Landroidx/media3/exoplayer/ExoPlayer$Builder;")
	if builder.campos[markerInerteNativo] == nil {
		t.Error("el builder de Media3 no quedó marcado como inerte")
	}
	reproductor := esperaObjetoDeStub(t, mustStub(t, m,
		[]string{"Landroidx/media3/exoplayer/ExoPlayer$Builder;"}, "build",
		"()Landroidx/media3/exoplayer/ExoPlayer;", nil, builder), "Landroidx/media3/exoplayer/ExoPlayer;")
	if reproductor.campos[markerInerteNativo] == nil {
		t.Error("el reproductor devuelto no quedó marcado como inerte")
	}
	item := mustStub(t, m, []string{"Landroidx/media3/common/MediaItem;"}, "fromUri",
		"(Ljava/lang/String;)Landroidx/media3/common/MediaItem;",
		[]any{cadenaObjeto("https://ejemplo.invalido")}, nil)
	esperaObjetoDeStub(t, item, "Landroidx/media3/common/MediaItem;")
	if v, ok, err := m.intentaStub([]string{"Landroidx/media3/common/Player;"}, "setMediaItem",
		"(Landroidx/media3/common/MediaItem;)V", []any{item}, reproductor); err != nil || !ok || v != nil {
		t.Errorf("setMediaItem: v=%v ok=%v err=%v", v, ok, err)
	}
	if v := mustStub(t, m, []string{"Landroidx/media3/common/Player;"}, "getCurrentPosition",
		"()J", nil, reproductor); v != int64(0) {
		t.Errorf("getCurrentPosition devolvió %v y se esperaba 0", v)
	}
	esperaObjetoDeStub(t, mustStub(t, m, []string{"Landroidx/media3/common/Player;"}, "getMediaItem",
		"()Landroidx/media3/common/MediaItem;", nil, reproductor), "Landroidx/media3/common/MediaItem;")
	// la generación anterior (ExoPlayer 2) se atiende con la misma familia
	antiguo := esperaObjetoDeStub(t, mustStub(t, m,
		[]string{"Lcom/google/android/exoplayer2/ExoPlayerFactory;"}, "newSimpleInstance",
		"(Landroid/content/Context;)Lcom/google/android/exoplayer2/ExoPlayer;",
		[]any{objDe("Landroid/app/Activity;")}, nil), "Lcom/google/android/exoplayer2/ExoPlayer;")
	if antiguo.campos[markerInerteNativo] == nil {
		t.Error("el ExoPlayer 2 devuelto no quedó marcado como inerte")
	}
	avisos := strings.Join(m.avisos, "\n")
	for _, esperado := range []string{
		"nativo-inerte: androidx.media3.exoplayer.ExoPlayer$Builder.Builder",
		"nativo-inerte: androidx.media3.exoplayer.ExoPlayer$Builder.build",
		"nativo-inerte: androidx.media3.common.MediaItem.fromUri",
		"nativo-inerte: androidx.media3.exoplayer.ExoPlayer.setMediaItem",
		"nativo-inerte: androidx.media3.exoplayer.ExoPlayer.getCurrentPosition",
		"nativo-inerte: androidx.media3.exoplayer.ExoPlayer.getMediaItem",
		"nativo-inerte: com.google.android.exoplayer2.ExoPlayerFactory.newSimpleInstance",
	} {
		if !strings.Contains(avisos, esperado) {
			t.Errorf("falta el aviso %q en %q", esperado, m.avisos)
		}
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("la cadena ExoPlayer cayó en el aviso genérico: %q", m.avisos)
	}
}

// TestInertesSinCargaNativaNiRedEnElCodigo garantiza la frontera en el propio origen: el intérprete
// no importa paquetes de red, procesos, carga dinámica ni cgo, y no contiene llamadas a ellos. Sin
// esta prueba, un «no pedimos red» sería solo una frase en un comentario.
func TestInertesSinCargaNativaNiRedEnElCodigo(t *testing.T) {
	datos, err := os.ReadFile("previewinterprete.go")
	if err != nil {
		t.Fatalf("no se pudo leer el intérprete: %v", err)
	}
	texto := string(datos)
	ini := strings.Index(texto, "import (")
	fin := strings.Index(texto, "\n)")
	if ini < 0 || fin < ini {
		t.Fatal("no se encontró el bloque de importaciones del intérprete")
	}
	imports := texto[ini:fin]
	for _, prohibido := range []string{`"os"`, `"net"`, `"net/http"`, `"os/exec"`,
		`"plugin"`, `"syscall"`, `"unsafe"`, `"C"`} {
		if strings.Contains(imports, prohibido) {
			t.Errorf("el intérprete importa %s, fuera de la frontera inerte", prohibido)
		}
	}
	for _, prohibido := range []string{"http.Get", "http.Post", "net.Dial", "exec.Command",
		"os.Open", "syscall.", "plugin.Open"} {
		if strings.Contains(texto, prohibido) {
			t.Errorf("el intérprete contiene %s, que cargaría código o pediría red", prohibido)
		}
	}
}

// TestInertesMarcaAvisosRecortados falsifica el tope con llamadas inertes DISTINTAS: los 24 primeros
// avisos caben, los tres siguientes se recortan y el modelo tiene que decir cuántos faltan. Sin el
// marcador, un registro recortado parecería completo.
func TestInertesMarcaAvisosRecortados(t *testing.T) {
	m := maquinaDeStubs()
	vista := objDe("Landroid/webkit/WebView;")
	vista.campos[markerInerteWeb] = vista.clase
	for i := 0; i < avisosMaximos+3; i++ {
		nombre := fmt.Sprintf("metodoInerte%02d", i)
		v, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, nombre, "()V", nil, vista)
		if err != nil || !ok || v != nil {
			t.Fatalf("%s: v=%v ok=%v err=%v", nombre, v, ok, err)
		}
	}
	if m.avisosRecortados != 3 {
		t.Errorf("se recortaron %d avisos y se esperaban 3", m.avisosRecortados)
	}
	cabidos := 0
	for _, aviso := range m.avisos {
		if strings.HasPrefix(aviso, "webview-inerte: android.webkit.WebView.metodoInerte") {
			cabidos++
		}
	}
	if cabidos != avisosMaximos {
		t.Errorf("quedan %d avisos de la app y se esperaban los %d del tope", cabidos, avisosMaximos)
	}
	ultimo := m.avisos[len(m.avisos)-1]
	if ultimo != "avisos-recortados: 3" {
		t.Errorf("el último aviso es %q y se esperaba el marcador", ultimo)
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "avisos-recortados: 3") {
		t.Errorf("no aparece el marcador en %q", m.avisos)
	}
	// el tope sigue siendo el mismo: el marcador no abre hueco para un aviso nuevo de la app
	if _, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, "metodoInerteTardio",
		"()V", nil, vista); err != nil || !ok {
		t.Fatalf("metodoInerteTardio: ok=%v err=%v", ok, err)
	}
	if m.avisosRecortados != 4 || m.avisos[len(m.avisos)-1] != "avisos-recortados: 4" {
		t.Errorf("el marcador no se actualizó: recortados=%d último=%q", m.avisosRecortados,
			m.avisos[len(m.avisos)-1])
	}
}

// TestInertesRetornoArregloEsVacio comprueba el retorno de tipo arreglo del fallback: es un objeto
// con la clase del arreglo y `lista` vacía, y `array-length` lo lee como cero en vez de fallar con
// «no es arreglo». El índice cero, al estar vacío, da un error de rango honesto en `aget`.
func TestInertesRetornoArregloEsVacio(t *testing.T) {
	m := maquinaDeStubs()
	reproductor := objDe("Landroidx/media3/exoplayer/ExoPlayer;")
	reproductor.campos[markerInerteNativo] = reproductor.clase
	arreglo := mustStub(t, m, []string{"Landroidx/media3/common/Player;"}, "getTrackList",
		"()[I", nil, reproductor)
	o, esObjeto := arreglo.(*objetoDalvik)
	if !esObjeto || o == nil {
		t.Fatalf("el retorno es %T y se esperaba un objeto de arreglo", arreglo)
	}
	if o.clase != "[I" {
		t.Errorf("la clase es %q y se esperaba [I", o.clase)
	}
	if o.lista == nil || len(o.lista) != 0 {
		t.Errorf("el arreglo inerte lleva %v y se esperaba una lista vacía no nula", o.lista)
	}
	if o.campos[markerInerteNativo] == nil {
		t.Error("el arreglo inerte perdió la marca de su familia")
	}
	// `array-length v0, v1` sobre el argumento, seguido de `return v0`
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	met := unMetodoDePrueba("longitud", &codeItem{registersSize: 2, insSize: 1, insnsSize: 2,
		insns: []uint16{0x1021, 0x000f}})
	n, err := m.ejecutaCuerpo("Lcom/prueba/A;", met, []any{o}, []string{"[I"})
	if err != nil {
		t.Fatalf("array-length sobre el arreglo inerte falló: %v", err)
	}
	if n != int32(0) {
		t.Errorf("array-length devolvió %v y se esperaba 0", n)
	}
	// `aget v0, v1, v2` con índice cero: el error tiene que ser de rango, no de tipo
	met = unMetodoDePrueba("lee", &codeItem{registersSize: 3, insSize: 2, insnsSize: 3,
		insns: []uint16{0x0044, 0x0201, 0x000e}})
	_, err = m.ejecutaCuerpo("Lcom/prueba/A;", met, []any{o, int32(0)}, []string{"[I", "I"})
	if err == nil || !strings.Contains(err.Error(), "fuera del arreglo de 0") {
		t.Errorf("aget sobre el arreglo vacío dio %v y se esperaba el error de rango", err)
	}
}

// TestClasesReflexionDevuelvenArregloVacio comprueba los dos arreglos que `ClassesInfoCache.createInfo`
// lee en el dex de NewPipe: getInterfaces y getDeclaredMethods devuelven un arreglo vacío NO nulo,
// `array-length` da cero y el aviso dice que es una aproximación. El guardián de receptor nulo de
// `invoca` sigue cortando antes de llegar al stub.
func TestClasesReflexionDevuelvenArregloVacio(t *testing.T) {
	casos := []struct {
		nombre string
		proto  string
		tipo   string
	}{
		{"getInterfaces", "()[Ljava/lang/Class;", "[Ljava/lang/Class;"},
		{"getDeclaredMethods", "()[Ljava/lang/reflect/Method;", "[Ljava/lang/reflect/Method;"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			m := maquinaDeStubs()
			m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
			clase := objDe("Ljava/lang/Class;")
			clase.campos["Ljava/lang/Class;.$tipo"] = "Landroidx/activity/ComponentActivity$$ExternalSyntheticLambda3;"
			arreglo := mustStub(t, m, []string{"Ljava/lang/Class;"}, caso.nombre, caso.proto, nil, clase)
			a, ok := arreglo.(*arregloDalvik)
			if !ok || a == nil {
				t.Fatalf("el retorno es %T y se esperaba un arreglo", arreglo)
			}
			if a.tipo != caso.tipo {
				t.Errorf("el tipo es %q y se esperaba %q", a.tipo, caso.tipo)
			}
			if a.datos == nil || len(a.datos) != 0 {
				t.Errorf("el arreglo lleva %v y se esperaban cero huecos no nulos", a.datos)
			}
			if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, caso.nombre) ||
				!strings.Contains(avisos, "aproximación") {
				t.Errorf("el aviso de aproximación falta en %q", avisos)
			}
			// `array-length v0, v1` seguido de `return v0`, como en createInfo
			met := unMetodoDePrueba("longitud", &codeItem{registersSize: 2, insSize: 1, insnsSize: 2,
				insns: []uint16{0x1021, 0x000f}})
			n, err := m.ejecutaCuerpo("Lcom/prueba/A;", met, []any{a}, []string{caso.tipo})
			if err != nil {
				t.Fatalf("array-length sobre el arreglo de reflexión falló: %v", err)
			}
			if n != int32(0) {
				t.Errorf("array-length devolvió %v y se esperaba 0", n)
			}
		})
	}
	m := maquinaDeStubs()
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/lang/Class;", name: "getInterfaces",
		proto: dexProto{returnType: "[Ljava/lang/Class;"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "")
	if err == nil || !strings.Contains(err.Error(), "java.lang.Class.getInterfaces sobre un objeto nulo") {
		t.Errorf("getInterfaces con receptor nulo dio %v y se esperaba el error del guardián", err)
	}
}

// TestInertesCuentaUnaVezAvisosRecortadosRepetidos falsifica el doble conteo: un aviso distinto que
// no cupo se marca como visto, así que cien llamadas al mismo método recortado cuentan UNO y no cien.
func TestInertesCuentaUnaVezAvisosRecortadosRepetidos(t *testing.T) {
	m := maquinaDeStubs()
	vista := objDe("Landroid/webkit/WebView;")
	vista.campos[markerInerteWeb] = vista.clase
	// se llena el tope con métodos distintos, sin recortar ninguno todavía
	for i := 0; i < avisosMaximos; i++ {
		nombre := fmt.Sprintf("metodoInerte%02d", i)
		if _, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, nombre,
			"()V", nil, vista); err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", nombre, ok, err)
		}
	}
	if m.avisosRecortados != 0 {
		t.Fatalf("el tope se recortó antes de tiempo: %d", m.avisosRecortados)
	}
	// el mismo método recortado, cien veces: solo la primera cuenta
	for i := 0; i < 100; i++ {
		if _, ok, err := m.intentaStub([]string{"Landroid/webkit/WebView;"}, "loadUrl",
			"(Ljava/lang/String;)V", []any{cadenaObjeto("https://ejemplo.invalido")}, vista); err != nil || !ok {
			t.Fatalf("loadUrl repetido %d: ok=%v err=%v", i, ok, err)
		}
	}
	if m.avisosRecortados != 1 {
		t.Errorf("se contaron %d avisos recortados y se esperaba 1", m.avisosRecortados)
	}
	marcadores := 0
	for _, aviso := range m.avisos {
		if strings.HasPrefix(aviso, "avisos-recortados: ") {
			marcadores++
		}
	}
	if marcadores != 1 {
		t.Errorf("hay %d marcadores y se esperaba uno: %q", marcadores, m.avisos)
	}
	if ultimo := m.avisos[len(m.avisos)-1]; ultimo != "avisos-recortados: 1" {
		t.Errorf("el último aviso es %q y se esperaba el marcador", ultimo)
	}
}

// --- mapas, cadenas y trazas que aparecieron tras ejecutar <clinit> ---------------------------
//
// Los cinco métodos de abajo llegaban a la fase 3 de `invoca` («no está en el dex ni en los
// stubs») con el frente de clinit (evidencia en clinit/salida2) y, en el de lifecycle, el getter de
// StackTraceElement era el único aviso vivo de Libre Memory Game (`lifecycle/post`). Se comprueban
// directamente sobre `intentaStub`, sin dex ni compilación: la semántica es sobre los objetos que el
// intérprete ya modela, no sobre un jar ajeno.

// TestColeccionesMapGetLeeLaEntradaSinInventar: la entrada sembrada se devuelve y la ausente es
// `null` con un aviso que dice que es una aproximación, nunca el cero silencioso de antes.
func TestColeccionesMapGetLeeLaEntradaSinInventar(t *testing.T) {
	m := maquinaDeStubs()
	mapa := objDe("Ljava/util/HashMap;")
	mapa.campos["mapa.clave"] = cadenaObjeto("valor")
	candidatos := []string{"Ljava/util/Map;"} // el invoke declara la interfaz

	v, ok, err := m.intentaStub(candidatos, "get", "(Ljava/lang/Object;)Ljava/lang/Object;",
		[]any{cadenaObjeto("clave")}, mapa)
	if err != nil || !ok {
		t.Fatalf("get de una entrada sembrada: v=%v ok=%v err=%v", v, ok, err)
	}
	esperaCadenaDeStub(t, v, "valor")

	v, ok, err = m.intentaStub(candidatos, "get", "(Ljava/lang/Object;)Ljava/lang/Object;",
		[]any{cadenaObjeto("ausente")}, mapa)
	if err != nil || !ok {
		t.Fatalf("get de una entrada ausente: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != nil {
		t.Errorf("una clave ausente devolvió %v y se esperaba el receptor nulo", v)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "aproximación") {
		t.Errorf("la clave ausente no registró que es una aproximación: %s", avisos)
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("Map.get cayó en el aviso genérico: %s", avisos)
	}
}

// TestCadenasSubstringCuentaUnidadesYRespetaLimites: recorta en unidades UTF-16, acepta las dos
// firmas que usa javac (`substring(I)` y `substring(II)`) y lanza la excepción del framework cuando
// el rango no es válido, para que un `catch` del proyecto pueda seguir.
func TestCadenasSubstringCuentaUnidadesYRespetaLimites(t *testing.T) {
	m := maquinaDeStubs()
	candidatos := []string{"Ljava/lang/String;"}

	casos := []struct {
		receptor any
		vals     []any
		esperado string
	}{
		{cadenaObjeto("abcdef"), []any{int32(2)}, "cdef"},
		{"abcdef", []any{int32(1), int32(4)}, "bcd"},
		{cadenaObjeto("aéb"), []any{int32(1), int32(2)}, "é"},
		{"abcdef", []any{int32(2), int32(2)}, ""},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(candidatos, "substring", "(I)Ljava/lang/String;", c.vals, c.receptor)
		if err != nil || !ok {
			t.Errorf("substring(%v) sobre %v: v=%v ok=%v err=%v", c.vals, c.receptor, v, ok, err)
			continue
		}
		esperaCadenaDeStub(t, v, c.esperado)
	}

	malos := [][]any{{int32(-1)}, {int32(1), int32(0)}, {int32(1), int32(9)}}
	for _, vals := range malos {
		v, ok, err := m.intentaStub(candidatos, "substring", "(I)Ljava/lang/String;", vals, "abc")
		if !ok {
			t.Errorf("substring(%v) no fue atendido", vals)
			continue
		}
		var ex *excepcionDalvik
		if !errors.As(err, &ex) {
			t.Errorf("substring(%v) devolvió v=%v err=%v y se esperaba una excepción", vals, v, err)
			continue
		}
		if ex.tipo != "Ljava/lang/StringIndexOutOfBoundsException;" {
			t.Errorf("substring(%v) lanzó %s y se esperaba StringIndexOutOfBoundsException", vals, ex.tipo)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("substring cayó en el aviso genérico: %s", avisos)
	}
}

// TestCadenasIntegerValueOfYIntValue: el boxeo devuelve un Integer no nulo con el valor guardado,
// que `intValue` lee sin caer en el guardián de receptor nulo del invoke siguiente.
func TestCadenasIntegerValueOfYIntValue(t *testing.T) {
	m := maquinaDeStubs()
	candidatos := []string{"Ljava/lang/Integer;"}

	v, ok, err := m.intentaStub(candidatos, "valueOf", "(I)Ljava/lang/Integer;", []any{int32(42)}, nil)
	if err != nil || !ok {
		t.Fatalf("Integer.valueOf: v=%v ok=%v err=%v", v, ok, err)
	}
	entero := esperaObjetoDeStub(t, v, "Ljava/lang/Integer;")

	v, ok, err = m.intentaStub(candidatos, "intValue", "()I", nil, entero)
	if err != nil || !ok {
		t.Fatalf("Integer.intValue: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != int32(42) {
		t.Errorf("intValue devolvió %v y se esperaba 42", v)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("el boxeo cayó en el aviso genérico: %s", avisos)
	}
}

// TestStackTraceGetStackTraceYGetters: respeta la traza ya sembrada; sin ella, entrega una casilla
// no nula con clase, método y línea para que `getClassName` no devuelva un nombre vacío que reviente
// el `HashMap.get` o el `equals` siguientes (medido en Libre Memory Game).
func TestStackTraceGetStackTraceYGetters(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	candidatos := []string{"Ljava/lang/Throwable;"}
	proto := "()[Ljava/lang/StackTraceElement;"

	elemento := &objetoDalvik{clase: "Ljava/lang/StackTraceElement;", campos: map[string]any{
		"StackTraceElement.clase":  "com.ejemplo.Main",
		"StackTraceElement.metodo": "onCreate",
		"StackTraceElement.linea":  int32(17),
	}}
	sembrada := &arregloDalvik{tipo: "[Ljava/lang/StackTraceElement;", datos: []any{elemento}}
	ex := objDe("Ljava/lang/IllegalStateException;")
	ex.campos["throwable.stackTrace"] = sembrada

	v, ok, err := m.intentaStub(candidatos, "getStackTrace", proto, nil, ex)
	if err != nil || !ok {
		t.Fatalf("getStackTrace con traza sembrada: v=%v ok=%v err=%v", v, ok, err)
	}
	arreglo, esArreglo := v.(*arregloDalvik)
	if !esArreglo || arreglo != sembrada {
		t.Fatalf("getStackTrace devolvió %T y se esperaba la traza sembrada", v)
	}

	getters := []struct {
		nombre   string
		proto    string
		esperado any
	}{
		{"getClassName", "()Ljava/lang/String;", "com.ejemplo.Main"},
		{"getMethodName", "()Ljava/lang/String;", "onCreate"},
		{"getLineNumber", "()I", int32(17)},
	}
	for _, g := range getters {
		v, ok, err := m.intentaStub([]string{"Ljava/lang/StackTraceElement;"}, g.nombre, g.proto, nil, elemento)
		if err != nil || !ok {
			t.Errorf("%s: v=%v ok=%v err=%v", g.nombre, v, ok, err)
			continue
		}
		if texto, esTexto := g.esperado.(string); esTexto {
			esperaCadenaDeStub(t, v, texto)
		} else if v != g.esperado {
			t.Errorf("%s devolvió %v y se esperaba %v", g.nombre, v, g.esperado)
		}
	}

	// sin traza sembrada: una casilla sintética, con aviso explícito de aproximación
	otra := objDe("Ljava/lang/RuntimeException;")
	v, ok, err = m.intentaStub(candidatos, "getStackTrace", proto, nil, otra)
	if err != nil || !ok {
		t.Fatalf("getStackTrace sin traza: v=%v ok=%v err=%v", v, ok, err)
	}
	sintetica, esArreglo := v.(*arregloDalvik)
	if !esArreglo || sintetica == nil || sintetica.tipo != protoReturn(proto) || len(sintetica.datos) != 1 {
		t.Fatalf("la traza sintética no es un arreglo de una casilla: %T %v", v, v)
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "aproximación") {
		t.Errorf("la traza sintética no registró que es una aproximación: %v", m.avisos)
	}
	if _, ok := sintetica.datos[0].(*objetoDalvik); !ok {
		t.Errorf("la casilla sintética es %T y se esperaba un StackTraceElement", sintetica.datos[0])
	}
}

// protoReturn es el descriptor del arreglo que devuelve getStackTrace, sin depender de un dex.
func protoReturn(proto string) string {
	return strings.TrimPrefix(proto, "()")
}

// TestStackTraceGettersConCamposVacios falsifica la casilla que sembró el frente de lifecycle: un
// StackTraceElement sin campos debe leerse como cadena vacía y línea cero, nunca como un objeto
// nulo que reviente el siguiente `equals`, y sin dejar un aviso nuevo.
func TestStackTraceGettersConCamposVacios(t *testing.T) {
	m := maquinaDeStubs()
	elemento := objDe("Ljava/lang/StackTraceElement;")
	candidatos := []string{"Ljava/lang/StackTraceElement;"}
	casos := []struct {
		nombre   string
		proto    string
		esperado any
	}{
		{"getClassName", "()Ljava/lang/String;", ""},
		{"getMethodName", "()Ljava/lang/String;", ""},
		{"getLineNumber", "()I", int32(0)},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(candidatos, c.nombre, c.proto, nil, elemento)
		if err != nil || !ok {
			t.Fatalf("%s con campos vacíos: v=%v ok=%v err=%v", c.nombre, v, ok, err)
		}
		if v == nil {
			t.Errorf("%s con campos vacíos devolvió el receptor nulo", c.nombre)
			continue
		}
		if texto, esTexto := c.esperado.(string); esTexto {
			esperaCadenaDeStub(t, v, texto)
		} else if v != c.esperado {
			t.Errorf("%s con campos vacíos devolvió %v y se esperaba %v", c.nombre, v, c.esperado)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); avisos != "" {
		t.Errorf("los getters con campos vacíos registraron avisos: %s", avisos)
	}
}

// TestCadenasSubstringCortaParesSurrogate: un emoji son dos unidades de 16 bits y una sola runa.
// Si `substring` contara runas, el corte (1,2) lanzaría StringIndexOutOfBoundsException; al cortar
// unidades devuelve una casilla suelta. El par completo y el fin también se comprueban.
func TestCadenasSubstringCortaParesSurrogate(t *testing.T) {
	m := maquinaDeStubs()
	candidatos := []string{"Ljava/lang/String;"}
	emoji := "😀"
	if unidades := unidadesDeCadena(emoji); len(unidades) != 2 {
		t.Fatalf("el emoji tiene %d unidades y se esperaban 2", len(unidades))
	}
	casos := []struct {
		vals     []any
		unidades int
	}{
		{[]any{int32(0), int32(1)}, 1},
		{[]any{int32(1), int32(2)}, 1},
		{[]any{int32(0), int32(2)}, 2},
		{[]any{int32(2), int32(2)}, 0},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub(candidatos, "substring", "(II)Ljava/lang/String;", c.vals, emoji)
		if err != nil || !ok {
			t.Errorf("substring(%v): v=%v ok=%v err=%v", c.vals, v, ok, err)
			continue
		}
		s, esCadena := cadenaDe(v)
		if !esCadena {
			t.Errorf("substring(%v) devolvió %T y se esperaba una cadena", c.vals, v)
			continue
		}
		if n := len(unidadesDeCadena(s)); n != c.unidades {
			t.Errorf("substring(%v) devolvió %d unidades y se esperaban %d", c.vals, n, c.unidades)
		}
	}
	v, ok, err := m.intentaStub(candidatos, "substring", "(II)Ljava/lang/String;",
		[]any{int32(0), int32(3)}, emoji)
	if !ok {
		t.Fatal("substring(0,3) sobre un emoji no fue atendido")
	}
	var ex *excepcionDalvik
	if !errors.As(err, &ex) {
		t.Errorf("substring(0,3) devolvió v=%v err=%v y se esperaba una excepción", v, err)
	} else if ex.tipo != "Ljava/lang/StringIndexOutOfBoundsException;" {
		t.Errorf("substring(0,3) lanzó %s y se esperaba StringIndexOutOfBoundsException", ex.tipo)
	}
}

// TestArraysAsListCopiaLosElementosYEsInmutable: la lista no comparte el arreglo original y rechaza
// add/clear con la excepción del framework, en vez de mutarse en silencio.
func TestArraysAsListCopiaLosElementosYEsInmutable(t *testing.T) {
	m := maquinaDeStubs()
	arr := &arregloDalvik{tipo: "[Ljava/lang/Object;", datos: []any{cadenaObjeto("a"), int32(2)}}
	candidatos := []string{"Ljava/util/Arrays;"}

	v, ok, err := m.intentaStub(candidatos, "asList",
		"([Ljava/lang/Object;)Ljava/util/List;", []any{arr}, nil)
	if err != nil || !ok {
		t.Fatalf("Arrays.asList: v=%v ok=%v err=%v", v, ok, err)
	}
	lista := esperaObjetoDeStub(t, v, "Ljava/util/ArrayList;")
	if len(lista.lista) != 2 {
		t.Fatalf("la lista tiene %d elementos y se esperaban 2", len(lista.lista))
	}
	if !boolDe(lista.campos["lista.inmutable"]) {
		t.Error("la lista no quedó marcada como inmutable")
	}
	arr.datos[0] = cadenaObjeto("z")
	esperaCadenaDeStub(t, lista.lista[0], "a")

	for _, caso := range []struct {
		nombre string
		proto  string
		vals   []any
	}{
		{"add", "(Ljava/lang/Object;)Z", []any{cadenaObjeto("x")}},
		{"clear", "()V", nil},
	} {
		_, ok, err := m.intentaStub([]string{"Ljava/util/ArrayList;"}, caso.nombre, caso.proto,
			caso.vals, lista)
		if !ok {
			t.Errorf("%s sobre la lista inmutable no fue atendido", caso.nombre)
			continue
		}
		var ex *excepcionDalvik
		if !errors.As(err, &ex) {
			t.Errorf("%s devolvió err=%v y se esperaba UnsupportedOperationException", caso.nombre, err)
		} else if ex.tipo != "Ljava/lang/UnsupportedOperationException;" {
			t.Errorf("%s lanzó %s y se esperaba UnsupportedOperationException", caso.nombre, ex.tipo)
		}
	}
	if len(lista.lista) != 2 {
		t.Errorf("add/clear mutaron la lista: quedan %d elementos", len(lista.lista))
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("Arrays.asList cayó en el aviso genérico: %s", avisos)
	}
}

// TestArraysCopyOfRangeRespetaLimites: copia el rango pedido, rellena con null si `hasta` pasa de
// la longitud y lanza la excepción del framework si el rango va hacia atrás o empieza antes del cero.
func TestArraysCopyOfRangeRespetaLimites(t *testing.T) {
	m := maquinaDeStubs()
	arr := &arregloDalvik{tipo: "[Ljava/lang/Object;", datos: []any{int32(1), int32(2), int32(3)}}
	candidatos := []string{"Ljava/util/Arrays;"}
	proto := "([Ljava/lang/Object;II)[Ljava/lang/Object;"

	v, ok, err := m.intentaStub(candidatos, "copyOfRange", proto,
		[]any{arr, int32(1), int32(3)}, nil)
	if err != nil || !ok {
		t.Fatalf("copyOfRange(1,3): v=%v ok=%v err=%v", v, ok, err)
	}
	copia, esArreglo := v.(*arregloDalvik)
	if !esArreglo || copia.tipo != arr.tipo || len(copia.datos) != 2 {
		t.Fatalf("la copia es %T (%v) y se esperaba un arreglo de dos elementos", v, v)
	}
	if copia.datos[0] != int32(2) || copia.datos[1] != int32(3) {
		t.Errorf("la copia trae %v y se esperaba [2 3]", copia.datos)
	}

	v, ok, err = m.intentaStub(candidatos, "copyOfRange", proto,
		[]any{arr, int32(1), int32(5)}, nil)
	if err != nil || !ok {
		t.Fatalf("copyOfRange(1,5): v=%v ok=%v err=%v", v, ok, err)
	}
	relleno := v.(*arregloDalvik)
	if len(relleno.datos) != 4 || relleno.datos[2] != nil || relleno.datos[3] != nil {
		t.Errorf("el relleno con null salió mal: %v", relleno.datos)
	}

	// Los tipos y los mensajes son los del JDK 17, medidos con CheckRange.java (y CheckNeg.java
	// para el origen negativo): un rango invertido NO es ArrayIndexOutOfBoundsException.
	casosMalos := []struct {
		vals    []any
		tipo    string
		mensaje string
	}{
		{[]any{arr, int32(-1), int32(2)}, "Ljava/lang/ArrayIndexOutOfBoundsException;",
			"arraycopy: source index -1 out of bounds for object array[3]"},
		{[]any{arr, int32(2), int32(1)}, "Ljava/lang/IllegalArgumentException;", "2 > 1"},
		{[]any{arr, int32(5), int32(6)}, "Ljava/lang/ArrayIndexOutOfBoundsException;",
			"arraycopy: length -2 is negative"},
	}
	for _, c := range casosMalos {
		v, ok, err := m.intentaStub(candidatos, "copyOfRange", proto, c.vals, nil)
		if !ok {
			t.Errorf("copyOfRange(%v) no fue atendido", c.vals)
			continue
		}
		var ex *excepcionDalvik
		if !errors.As(err, &ex) {
			t.Errorf("copyOfRange(%v) devolvió v=%v err=%v y se esperaba una excepción", c.vals, v, err)
			continue
		}
		if ex.tipo != c.tipo {
			t.Errorf("copyOfRange(%v) lanzó %s y se esperaba %s", c.vals, ex.tipo, c.tipo)
		}
		if ex.objeto != nil && ex.objeto.mensajeExcepcion != c.mensaje {
			t.Errorf("copyOfRange(%v) dijo %q y se esperaba %q", c.vals,
				ex.objeto.mensajeExcepcion, c.mensaje)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("Arrays.copyOfRange cayó en el aviso genérico: %s", avisos)
	}
}

// TestIteradorDelSetRecorreClavesOrdenadas reutiliza `clavesDelMapa`: el mismo iterador de siempre
// entrega las claves que el objeto ya guardó, en orden determinista y sin inventar ninguna.
func TestIteradorDelSetRecorreClavesOrdenadas(t *testing.T) {
	m := maquinaDeStubs()
	set := objDe("Ljava/util/Set;")
	set.campos["mapa.b"] = int32(2)
	set.campos["mapa.a"] = int32(1)

	v, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "iterator",
		"()Ljava/util/Iterator;", nil, set)
	if err != nil || !ok {
		t.Fatalf("Set.iterator: v=%v ok=%v err=%v", v, ok, err)
	}
	it := esperaObjetoDeStub(t, v, "Ljava/util/Iterator;")
	for _, esperado := range []string{"a", "b"} {
		siguiente, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "next",
			"()Ljava/lang/Object;", nil, it)
		if err != nil || !ok {
			t.Fatalf("next: v=%v ok=%v err=%v", siguiente, ok, err)
		}
		esperaCadenaDeStub(t, siguiente, esperado)
	}
	hay, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok || hay != int32(0) {
		t.Errorf("hasNext al final devolvió %v ok=%v err=%v y se esperaba 0", hay, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("Set.iterator cayó en el aviso genérico: %s", avisos)
	}
}

// TestIteradorDelSetVacioNoEsNuelo falsifica el caso de Termux: un HashSet sin claves guardadas
// devuelve un iterador no nulo que no recorre nada, nunca el cero ni una clave inventada.
func TestIteradorDelSetVacioNoEsNulo(t *testing.T) {
	m := maquinaDeStubs()
	vacio := objDe("Ljava/util/HashSet;")
	v, ok, err := m.intentaStub([]string{"Ljava/util/HashSet;", "Ljava/util/Set;"},
		"iterator", "()Ljava/util/Iterator;", nil, vacio)
	if err != nil || !ok {
		t.Fatalf("iterator sobre un HashSet vacío: v=%v ok=%v err=%v", v, ok, err)
	}
	it := esperaObjetoDeStub(t, v, "Ljava/util/Iterator;")
	if it.iterando == nil || len(it.iterando) != 0 {
		t.Errorf("el iterador vacío trae %v y se esperaba una lista vacía no nula", it.iterando)
	}
	hay, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok || hay != int32(0) {
		t.Errorf("hasNext devolvió %v ok=%v err=%v y se esperaba 0", hay, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("el conjunto vacío cayó en el aviso genérico: %s", avisos)
	}
}

// TestStackTraceSetStackTraceGuardaLaTraza comprueba el camino de ida y vuelta: lo que la app pone
// con setStackTrace es exactamente lo que devuelve getStackTrace, sin una casilla inventada.
func TestStackTraceSetStackTraceGuardaLaTraza(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	ex := objDe("Ljava/lang/IllegalStateException;")
	elemento := &objetoDalvik{clase: "Ljava/lang/StackTraceElement;", campos: map[string]any{
		"StackTraceElement.clase": "com.ejemplo.Main",
	}}
	traza := &arregloDalvik{tipo: "[Ljava/lang/StackTraceElement;", datos: []any{elemento}}
	candidatos := []string{"Ljava/lang/Throwable;"}

	v, ok, err := m.intentaStub(candidatos, "setStackTrace",
		"([Ljava/lang/StackTraceElement;)V", []any{traza}, ex)
	if err != nil || !ok || v != nil {
		t.Fatalf("setStackTrace: v=%v ok=%v err=%v", v, ok, err)
	}
	if guardada, ok := ex.campos["throwable.stackTrace"].(*arregloDalvik); !ok || guardada != traza {
		t.Fatalf("la traza guardada es %T y se esperaba la misma instancia", ex.campos["throwable.stackTrace"])
	}
	v, ok, err = m.intentaStub(candidatos, "getStackTrace",
		"()[Ljava/lang/StackTraceElement;", nil, ex)
	if err != nil || !ok {
		t.Fatalf("getStackTrace: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != traza {
		t.Errorf("getStackTrace devolvió %T y se esperaba la traza guardada", v)
	}
	if avisos := strings.Join(m.avisos, "\n"); avisos != "" {
		t.Errorf("el camino set/get registró avisos: %s", avisos)
	}
}

// --- familia 6: propiedades deterministas, sin ficheros ni sistema ------------------------------

// TestPropertiesTermuxDevuelveUnSetNoNulo falsifica el error que detenía a Termux: el cero de
// `stringPropertyNames` llegaba a `Set.iterator()` y el intérprete abortaba con «sobre un objeto
// nulo». El conjunto tiene que existir aunque la app no haya declarado ninguna clave ni llamado a
// `<init>`: la vista previa no lee ficheros y no tiene sistema de propiedades.
func TestPropertiesTermuxDevuelveUnSetNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	props := objDe("Ljava/util/Properties;")

	conjunto, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"},
		"stringPropertyNames", "()Ljava/util/Set;", nil, props)
	if err != nil || !ok || conjunto == nil {
		t.Fatalf("stringPropertyNames: v=%v ok=%v err=%v", conjunto, ok, err)
	}
	set := esperaObjetoDeStub(t, conjunto, "Ljava/util/Set;")
	if set.conjunto == nil {
		t.Fatal("stringPropertyNames devolvió un Set con el conjunto nulo")
	}
	if len(set.conjunto) != 0 {
		t.Errorf("el conjunto vacío tiene %d claves: %v", len(set.conjunto), set.conjunto)
	}

	it, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "iterator", "()Ljava/util/Iterator;",
		nil, set)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator del Set: v=%v ok=%v err=%v", it, ok, err)
	}
	esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
	hasNext, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok {
		t.Fatalf("hasNext del Set vacío: ok=%v err=%v", ok, err)
	}
	if hasNext != int32(0) {
		t.Errorf("hasNext devolvió %v y el conjunto está vacío", hasNext)
	}

	// la iteración directa sobre el Properties también entrega el conjunto de claves, sin receptor
	// nulo y sin depender de que la app haya ejecutado su constructor.
	it, ok, err = m.intentaStub([]string{"Ljava/util/Properties;"}, "iterator",
		"()Ljava/util/Iterator;", nil, props)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator del Properties: v=%v ok=%v err=%v", it, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna llamada cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestPropertiesConClavesDeclaradasSonDeterministas comprueba que las claves que la app escribe con
// `setProperty` se recorren en orden alfabético, no en el orden azaroso del mapa de Go, y que las
// lecturas devuelven su valor. Dos ejecuciones del mismo guion tienen que dar el mismo modelo.
func TestPropertiesConClavesDeclaradasSonDeterministas(t *testing.T) {
	m := maquinaDeStubs()
	props := objDe("Ljava/util/Properties;")
	declaradas := []struct{ clave, valor string }{
		{"z", "ultima"}, {"a", "primera"}, {"m", "media"},
	}
	for _, d := range declaradas {
		v, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "setProperty",
			"(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/Object;",
			[]any{cadenaObjeto(d.clave), cadenaObjeto(d.valor)}, props)
		if err != nil || !ok {
			t.Fatalf("setProperty(%q): ok=%v err=%v", d.clave, ok, err)
		}
		if v != props {
			t.Errorf("setProperty no devolvió el Properties: %v", v)
		}
	}

	conjunto, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "stringPropertyNames",
		"()Ljava/util/Set;", nil, props)
	if err != nil || !ok {
		t.Fatalf("stringPropertyNames: ok=%v err=%v", ok, err)
	}
	set := esperaObjetoDeStub(t, conjunto, "Ljava/util/Set;")
	esperadas := []string{"a", "m", "z"}
	if len(set.conjunto) != len(esperadas) {
		t.Fatalf("el conjunto tiene %d claves y se esperaban %d", len(set.conjunto), len(esperadas))
	}
	for i, esperada := range esperadas {
		clave, esCadena := cadenaDe(set.conjunto[i])
		if !esCadena || clave != esperada {
			t.Errorf("la clave %d es %v y se esperaba %q", i, set.conjunto[i], esperada)
		}
	}

	for _, d := range declaradas {
		v, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "getProperty",
			"(Ljava/lang/String;)Ljava/lang/String;", []any{cadenaObjeto(d.clave)}, props)
		if err != nil || !ok {
			t.Fatalf("getProperty(%q): ok=%v err=%v", d.clave, ok, err)
		}
		esperaCadenaDeStub(t, v, d.valor)
	}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "getProperty",
		"(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;",
		[]any{cadenaObjeto("ausente"), cadenaObjeto("por defecto")}, props)
	if err != nil || !ok {
		t.Fatalf("getProperty con valor por defecto: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, v, "por defecto")

	// clear deja el mapa vacío, pero no nulo: el Set siguiente sigue siendo recorrible.
	if _, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "clear", "()V", nil, props); err != nil || !ok {
		t.Fatalf("clear: ok=%v err=%v", ok, err)
	}
	if _, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "stringPropertyNames",
		"()Ljava/util/Set;", nil, props); err != nil || !ok {
		t.Fatalf("stringPropertyNames tras clear: ok=%v err=%v", ok, err)
	}
	if props.propiedades == nil || len(props.propiedades) != 0 {
		t.Errorf("clear dejó el mapa en %v", props.propiedades)
	}
}

// TestSetEntrySetDePropertiesTermux falsifica la frontera de `TerminalColorScheme.updateWith`:
// Termux recorre `Properties.entrySet()` con `Set.iterator()`, y el cero del stub ausente llegaba
// como receptor nulo. El conjunto tiene que existir aunque no haya propiedades, y con claves
// declaradas debe entregar las entradas en el orden determinista de `clavesDelMapa`.
func TestSetEntrySetDePropertiesTermux(t *testing.T) {
	m := maquinaDeStubs()
	props := objDe("Ljava/util/Properties;")

	entradas, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"},
		"entrySet", "()Ljava/util/Set;", nil, props)
	if err != nil || !ok || entradas == nil {
		t.Fatalf("entrySet del Properties vacío: v=%v ok=%v err=%v", entradas, ok, err)
	}
	conjunto := esperaObjetoDeStub(t, entradas, "Ljava/util/Set;")
	if conjunto.conjunto == nil || len(conjunto.conjunto) != 0 {
		t.Fatalf("el conjunto vacío es %v", conjunto.conjunto)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "iterator",
		"()Ljava/util/Iterator;", nil, conjunto)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator del conjunto vacío: v=%v ok=%v err=%v", it, ok, err)
	}
	hasNext, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok || hasNext != int32(0) {
		t.Fatalf("hasNext del conjunto vacío: v=%v ok=%v err=%v", hasNext, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Fatalf("entrySet cayó en el aviso de stub ausente: %s", avisos)
	}

	// con propiedades declaradas, cada entrada lleva la clave y el valor que la app escribió, en
	// orden alfabético, y el cursor del Set recorre exactamente esas entradas.
	for _, d := range []struct{ clave, valor string }{
		{"cursor", "rojo"}, {"foreground", "blanco"}, {"background", "negro"},
	} {
		if _, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"}, "setProperty",
			"(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/Object;",
			[]any{cadenaObjeto(d.clave), cadenaObjeto(d.valor)}, props); err != nil || !ok {
			t.Fatalf("setProperty(%q): ok=%v err=%v", d.clave, ok, err)
		}
	}
	entradas, ok, err = m.intentaStub([]string{"Ljava/util/Properties;"},
		"entrySet", "()Ljava/util/Set;", nil, props)
	if err != nil || !ok || entradas == nil {
		t.Fatalf("entrySet con propiedades: v=%v ok=%v err=%v", entradas, ok, err)
	}
	conjunto = esperaObjetoDeStub(t, entradas, "Ljava/util/Set;")
	if len(conjunto.conjunto) != 3 {
		t.Fatalf("el conjunto tiene %d entradas y se esperaban 3", len(conjunto.conjunto))
	}
	it, ok, err = m.intentaStub([]string{"Ljava/util/Set;"}, "iterator",
		"()Ljava/util/Iterator;", nil, conjunto)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator con propiedades: v=%v ok=%v err=%v", it, ok, err)
	}
	esperadas := []struct{ clave, valor string }{
		{"background", "negro"}, {"cursor", "rojo"}, {"foreground", "blanco"},
	}
	for i, esperada := range esperadas {
		hasNext, ok, err = m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
		if err != nil || !ok || hasNext != int32(1) {
			t.Fatalf("hasNext %d: v=%v ok=%v err=%v", i, hasNext, ok, err)
		}
		entrada, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "next",
			"()Ljava/lang/Object;", nil, it)
		if err != nil || !ok {
			t.Fatalf("next %d: ok=%v err=%v", i, ok, err)
		}
		par := esperaObjetoDeStub(t, entrada, "Ljava/util/Map$Entry;")
		clave, ok, err := m.intentaStub([]string{"Ljava/util/Map$Entry;"}, "getKey",
			"()Ljava/lang/Object;", nil, par)
		if err != nil || !ok {
			t.Fatalf("getKey %d: ok=%v err=%v", i, ok, err)
		}
		esperaCadenaDeStub(t, clave, esperada.clave)
		valor, ok, err := m.intentaStub([]string{"Ljava/util/Map$Entry;"}, "getValue",
			"()Ljava/lang/Object;", nil, par)
		if err != nil || !ok {
			t.Fatalf("getValue %d: ok=%v err=%v", i, ok, err)
		}
		esperaCadenaDeStub(t, valor, esperada.valor)
	}
	hasNext, ok, err = m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok || hasNext != int32(0) {
		t.Fatalf("hasNext al final: v=%v ok=%v err=%v", hasNext, ok, err)
	}
}

// TestTermuxArraysToStringEsDeterminista comprueba el quinto aviso de Termux: el texto de un
// arreglo de Object, con null en los huecos vacíos y sin depender de la dirección del arreglo.
func TestTermuxArraysToStringEsDeterminista(t *testing.T) {
	m := maquinaDeStubs()
	arreglo := &arregloDalvik{tipo: "[Ljava/lang/Object;",
		datos: []any{cadenaObjeto("uno"), nil, int32(7)}}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Arrays;"}, "toString",
		"([Ljava/lang/Object;)Ljava/lang/String;", []any{arreglo}, nil)
	if err != nil || !ok {
		t.Fatalf("Arrays.toString: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, v, "[uno, null, 7]")
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("Arrays.toString cayó en el aviso de stub ausente: %s", avisos)
	}
}

// tablaDeSondaParaRecursos es un resources.arsc escrito con el mismo escritor del fork y leído con
// leeTabla: tres tipos (color, dimen y string) con un valor cada uno. Es la tabla a la que se debe
// enlazar el Resources sintético; no se inventa ningún valor fuera de ella.
func tablaDeSondaParaRecursos(t *testing.T) *tablaRecursos {
	t.Helper()
	tb := tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.prueba.recursos",
		global:        []string{"Hola"},
		globalUTF8:    true,
		nombresTipo:   []string{"color", "dimen", "string"},
		claves:        []string{"fondo", "margen", "saludo"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: []uint32{0x00000001}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 0, valor: valorEscribir{tipo: tipoColorRGB8, datos: 0x00336699}},
			}}}},
			{id: 2, specFlags: []uint32{0x00000001}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 1, valor: valorEscribir{tipo: tipoDimension, datos: 0x00001001}}, // 16dp
			}}}},
			{id: 3, specFlags: []uint32{0x00000001}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 2, valor: valorEscribir{tipo: tipoCadena, datos: 0}},
			}}}},
		},
	}
	datos, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla de la sonda: %v", err)
	}
	tr, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla escrita no se deja leer: %v", err)
	}
	return tr
}

// TestRecursosSinteticosDelContextDesbloqueaTermux falsifica la cadena que detenía a Termux: el cero
// de `Context.getResources` llegaba a `Resources.getDisplayMetrics` y el intérprete abortaba. El
// objeto entregado tiene que ser no nulo, estar enlazado a la tabla del APK y resolver solo lo que la
// tabla contiene.
func TestRecursosSinteticosDelContextDesbloqueaTermux(t *testing.T) {
	m := maquinaDeStubs()
	m.tabla = tablaDeSondaParaRecursos(t)
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	contexto := []string{"Lcom/termux/app/TermuxActivity;", "Landroid/content/Context;"}

	v, ok, err := m.intentaStub(contexto, "getResources", "()Landroid/content/res/Resources;", nil, actividad)
	if err != nil || !ok || v == nil {
		t.Fatalf("getResources: v=%v ok=%v err=%v", v, ok, err)
	}
	recursos := esperaObjetoDeStub(t, v, "Landroid/content/res/Resources;")
	if !esRecursosSinteticos(recursos) || tablaDeRecursosSinteticos(recursos) != m.tabla {
		t.Fatal("el Resources no está enlazado a la tabla del APK")
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "recursos-sinteticos:") {
		t.Errorf("getResources no dejó aviso de familia: %v", m.avisos)
	}

	metricas, ok, err := m.intentaStub([]string{"Landroid/content/res/Resources;"}, "getDisplayMetrics",
		"()Landroid/util/DisplayMetrics;", nil, recursos)
	if err != nil || !ok || metricas == nil {
		t.Fatalf("getDisplayMetrics: v=%v ok=%v err=%v", metricas, ok, err)
	}
	dm := esperaObjetoDeStub(t, metricas, "Landroid/util/DisplayMetrics;")
	if dm.campos["Landroid/util/DisplayMetrics;.widthPixels"] != int32(360) ||
		dm.campos["Landroid/util/DisplayMetrics;.heightPixels"] != int32(640) {
		t.Errorf("las métricas son %v", dm.campos)
	}

	casos := []struct {
		nombre string
		proto  string
		id     uint32
	}{
		{"getString", "(I)Ljava/lang/String;", 0x7f030000},
		{"getColor", "(I)I", 0x7f010000},
		{"getDimension", "(I)F", 0x7f020000},
	}
	esperados := []any{"Hola", int32(0x00336699), float32(16)}
	for i, c := range casos {
		valor, resuelto, err := m.intentaStub([]string{"Landroid/content/res/Resources;"}, c.nombre,
			c.proto, []any{int32(c.id)}, recursos)
		if err != nil || !resuelto {
			t.Fatalf("%s: v=%v ok=%v err=%v", c.nombre, valor, resuelto, err)
		}
		switch c.nombre {
		case "getString":
			esperaCadenaDeStub(t, valor, esperados[i].(string))
		case "getColor", "getDimension":
			if valor != esperados[i] {
				t.Errorf("%s devolvió %v y se esperaba %v", c.nombre, valor, esperados[i])
			}
		}
	}
}

// TestRecursosSinteticosNoInventanRecursos cierra la otra mitad del contrato: un id ausente en la
// tabla devuelve el cero (null para texto) y deja el aviso de familia, nunca un valor inventado.
func TestRecursosSinteticosNoInventanRecursos(t *testing.T) {
	m := maquinaDeStubs()
	m.tabla = tablaDeSondaParaRecursos(t)
	recursos := m.recursosSinteticos()
	const ausente = int32(0x7f990099)

	texto, ok, err := m.intentaStub([]string{"Landroid/content/res/Resources;"}, "getString",
		"(I)Ljava/lang/String;", []any{ausente}, recursos)
	if err != nil || !ok || texto != nil {
		t.Errorf("getString de un id ausente: v=%v ok=%v err=%v", texto, ok, err)
	}
	color, ok, err := m.intentaStub([]string{"Landroid/content/res/Resources;"}, "getColor",
		"(I)I", []any{ausente}, recursos)
	if err != nil || !ok || color != int32(0) {
		t.Errorf("getColor de un id ausente: v=%v ok=%v err=%v", color, ok, err)
	}
	dimen, ok, err := m.intentaStub([]string{"Landroid/content/res/Resources;"}, "getDimension",
		"(I)F", []any{ausente}, recursos)
	if err != nil || !ok || dimen != float32(0) {
		t.Errorf("getDimension de un id ausente: v=%v ok=%v err=%v", dimen, ok, err)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "recursos-sinteticos: getString(") ||
		!strings.Contains(avisos, "recursos-sinteticos: getColor(") ||
		!strings.Contains(avisos, "recursos-sinteticos: getDimension(") {
		t.Errorf("faltó el aviso de familia para algún id ausente: %v", m.avisos)
	}
}

// TestRecursosEstaticosDeTermuxNoNulos falsifica el `sget-object` que detenía a Termux en
// `getInternalTermuxPropertyValueFromValue`: sin `<clinit>`, los dos campos de
// TermuxPropertyConstants llegaban como null al `Set.contains` siguiente. El modelo entrega un Set
// vacío no nulo, avisa de la aproximación y no inventa ninguna clave de propiedad.
func TestRecursosEstaticosDeTermuxNoNulos(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}
	const dueño = "Lcom/termux/shared/settings/properties/TermuxPropertyConstants;"
	m.prog.clases[dueño] = &claseDalvik{estaticos: map[string]any{}}
	campos := []string{
		"TERMUX_DEFAULT_FALSE_BOOLEAN_BEHAVIOUR_PROPERTIES_LIST",
		"TERMUX_DEFAULT_TRUE_BOOLEAN_BEHAVIOUR_PROPERTIES_LIST",
	}
	for _, campo := range campos {
		v, err := m.leeEstatico(dexField{owner: dueño, name: campo, typeOf: "Ljava/util/Set;"})
		if err != nil || v == nil {
			t.Fatalf("leeEstatico(%s): v=%v err=%v", campo, v, err)
		}
		conjunto := esperaObjetoDeStub(t, v, "Ljava/util/Set;")
		if conjunto.conjunto == nil || len(conjunto.conjunto) != 0 {
			t.Errorf("%s entregó las claves %v y se esperaba un conjunto vacío", campo, conjunto.conjunto)
		}
		// el `Set.contains` siguiente se resuelve y responde falso sin receptor nulo.
		valor, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "contains",
			"(Ljava/lang/Object;)Z", []any{cadenaObjeto("extra-keys")}, conjunto)
		if err != nil || !ok || valor != int32(0) {
			t.Errorf("contains sobre %s: v=%v ok=%v err=%v", campo, valor, ok, err)
		}
		// la segunda lectura devuelve el mismo objeto: el estático quedó guardado en la clase.
		v2, err := m.leeEstatico(dexField{owner: dueño, name: campo, typeOf: "Ljava/util/Set;"})
		if err != nil || v2 != v {
			t.Errorf("la segunda lectura de %s devolvió %v (primera %v, err=%v)", campo, v2, v, err)
		}
	}
	// Falsificación del fallo encontrado en la APK: el `<clinit>` deja el campo guardado a nil y
	// esa entrada no debe ganarle al conjunto del modelo.
	guardado := dexField{owner: dueño, name: campos[0], typeOf: "Ljava/util/Set;"}
	m.prog.clases[dueño].estaticos[claveCampo(guardado)] = nil
	vGuardado, errGuardado := m.leeEstatico(guardado)
	if errGuardado != nil || vGuardado == nil {
		t.Errorf("lectura con el estático guardado a nil: v=%v err=%v", vGuardado, errGuardado)
	}

	avisos := strings.Join(m.avisos, "\n")
	for _, campo := range campos {
		if !strings.Contains(avisos, "aproximación-estática: com.termux.shared.settings.properties.TermuxPropertyConstants."+campo) {
			t.Errorf("falta el aviso de aproximación para %s: %s", campo, avisos)
		}
	}
}

// TestRecursosMapaInmutableDeTermux falsifica el `sget-object` que detenía a Termux en
// `setSessionShortcuts`: el campo MAP_SESSION_SHORTCUTS debe ser un ImmutableBiMap no nulo con
// entrySet/keySet/get sobre las claves literals del dex, sin inventar contenido.
func TestRecursosMapaInmutableDeTermux(t *testing.T) {
	m := maquinaDeStubs()
	const dueño = "Lcom/termux/shared/settings/properties/TermuxPropertyConstants;"
	m.prog = &programaDalvik{dex: &dexFile{strings: []string{
		"shortcut.create-session", "shortcut.next-session",
		"shortcut.previous-session", "shortcut.rename-session",
	}}, clases: map[string]*claseDalvik{}}
	m.prog.clases[dueño] = &claseDalvik{estaticos: map[string]any{}}
	campo := dexField{owner: dueño, name: "MAP_SESSION_SHORTCUTS",
		typeOf: "Lcom/google/common/collect/ImmutableBiMap;"}

	v, err := m.leeEstatico(campo)
	if err != nil || v == nil {
		t.Fatalf("leeEstatico(MAP_SESSION_SHORTCUTS): v=%v err=%v", v, err)
	}
	mapa := esperaObjetoDeStub(t, v, "Lcom/google/common/collect/ImmutableBiMap;")
	datos, _ := mapa.campos["mapa.datos"].(map[string]any)
	if len(datos) != 4 || datos["shortcut.next-session"] != int32(2) {
		t.Fatalf("el mapa del dex es %v", datos)
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "aproximación-estática:") {
		t.Errorf("el mapa no dejó aviso de aproximación: %v", m.avisos)
	}

	entradas, ok, err := m.intentaStub([]string{"Lcom/google/common/collect/ImmutableBiMap;"},
		"entrySet", "()Lcom/google/common/collect/ImmutableSet;", nil, mapa)
	if err != nil || !ok || entradas == nil {
		t.Fatalf("entrySet: v=%v ok=%v err=%v", entradas, ok, err)
	}
	conjunto := esperaObjetoDeStub(t, entradas, "Ljava/util/Set;")
	if len(conjunto.conjunto) != 4 {
		t.Fatalf("entrySet entregó %d entradas y se esperaban 4", len(conjunto.conjunto))
	}
	// el dex de Guava llama a createEntrySet, que no tiene cuerpo: su puente debe entregar el
	// mismo conjunto para que el `ImmutableSet.iterator()` siguiente no reciba un receptor nulo.
	puente, err := m.stubSinCuerpo("Lcom/google/common/collect/ImmutableBiMap;",
		&metodoDalvik{nombre: "createEntrySet", retorno: "Lcom/google/common/collect/ImmutableSet;"},
		[]any{mapa})
	if err != nil || puente == nil {
		t.Fatalf("createEntrySet sin cuerpo: v=%v err=%v", puente, err)
	}
	if n := len(esperaObjetoDeStub(t, puente, "Ljava/util/Set;").conjunto); n != 4 {
		t.Errorf("createEntrySet entregó %d entradas y se esperaban 4", n)
	}
	primera := esperaObjetoDeStub(t, conjunto.conjunto[0], "Ljava/util/Map$Entry;")
	clave, ok, err := m.intentaStub([]string{"Ljava/util/Map$Entry;"}, "getKey",
		"()Ljava/lang/Object;", nil, primera)
	if err != nil || !ok {
		t.Fatalf("getKey: v=%v ok=%v err=%v", clave, ok, err)
	}
	esperaCadenaDeStub(t, clave, "shortcut.create-session")
	valor, ok, err := m.intentaStub([]string{"Ljava/util/Map$Entry;"}, "getValue",
		"()Ljava/lang/Object;", nil, primera)
	if err != nil || !ok || valor != int32(1) {
		t.Errorf("getValue: v=%v ok=%v err=%v", valor, ok, err)
	}
	claves, ok, err := m.intentaStub([]string{"Lcom/google/common/collect/ImmutableBiMap;"},
		"keySet", "()Ljava/util/Set;", nil, mapa)
	if err != nil || !ok || claves == nil {
		t.Fatalf("keySet: v=%v ok=%v err=%v", claves, ok, err)
	}
	if n := len(esperaObjetoDeStub(t, claves, "Ljava/util/Set;").conjunto); n != 4 {
		t.Errorf("keySet entregó %d claves y se esperaban 4", n)
	}
	leido, ok, err := m.intentaStub([]string{"Lcom/google/common/collect/ImmutableBiMap;"}, "get",
		"(Ljava/lang/Object;)Ljava/lang/Object;", []any{cadenaObjeto("shortcut.next-session")}, mapa)
	if err != nil || !ok || leido != int32(2) {
		t.Errorf("get: v=%v ok=%v err=%v", leido, ok, err)
	}

	// una clave que no esté en el dex no se modela: no se inventa contenido.
	filtrado := maquinaDeStubs()
	filtrado.prog = &programaDalvik{dex: &dexFile{strings: []string{"shortcut.create-session"}},
		clases: map[string]*claseDalvik{dueño: {estaticos: map[string]any{}}}}
	v, err = filtrado.leeEstatico(campo)
	if err != nil || v == nil {
		t.Fatalf("leeEstatico filtrado: v=%v err=%v", v, err)
	}
	datos, _ = esperaObjetoDeStub(t, v, "Lcom/google/common/collect/ImmutableBiMap;").campos["mapa.datos"].(map[string]any)
	if len(datos) != 1 || datos["shortcut.create-session"] != int32(1) {
		t.Errorf("el mapa filtrado es %v", datos)
	}
}

// TestLayoutParamsSinteticoDeTermux falsifica el `iget LayoutParams.height` de
// `setTerminalToolbarView`: sin stub, `getLayoutParams` devolvía el cero y la lectura abortaba la
// ejecución en 6599 pasos. La vista se especializa con `check-cast` a ViewPager, igual que en el dex.
func TestLayoutParamsSinteticoDeTermux(t *testing.T) {
	m := maquinaDeStubs()
	vista := objDe("Landroidx/viewpager/widget/ViewPager;")
	proto := "()Landroid/view/ViewGroup$LayoutParams;"

	// fusila los field_ids del dex de Termux para pasar por `leeCampo`/`escribeCampo`, que son los
	// que ejecutan el iget y el iput reales.
	d := &dexFile{fields: []dexField{
		{owner: "android/view/ViewGroup$LayoutParams", name: "height", typeOf: "I"},
		{owner: "android/view/ViewGroup$LayoutParams", name: "width", typeOf: "I"},
	}}
	if _, err := m.leeCampo(d, 0, nil, false); err == nil ||
		!strings.Contains(err.Error(), "de un objeto nulo") {
		t.Fatalf("la lectura sobre nulo debería fallar, dio %v", err)
	}

	primero, ok, err := m.intentaStub([]string{"Landroid/view/View;"}, "getLayoutParams", proto,
		nil, vista)
	if err != nil || !ok {
		t.Fatalf("getLayoutParams: v=%v ok=%v err=%v", primero, ok, err)
	}
	params := esperaObjetoDeStub(t, primero, "Landroid/view/ViewGroup$LayoutParams;")
	if alto, _ := m.leeCampo(d, 0, params, false); alto != int32(0) {
		t.Errorf("el alto sintético es %v y se esperaba 0", alto)
	}
	if ancho, _ := m.leeCampo(d, 1, params, false); ancho != int32(0) {
		t.Errorf("el ancho sintético es %v y se esperaba 0", ancho)
	}
	if !strings.Contains(strings.Join(m.avisos, " "), "LayoutParams sintético") {
		t.Errorf("falta el aviso de aproximación: %v", m.avisos)
	}

	// el objeto se conserva: la altura que la app calcula después se escribe sobre el mismo.
	segundo, ok, err := m.intentaStub([]string{"Landroid/view/View;"}, "getLayoutParams", proto,
		nil, vista)
	if err != nil || !ok || segundo != primero {
		t.Fatalf("la segunda lectura no devolvió el mismo objeto: %v %v %v", segundo, ok, err)
	}
	if err := m.escribeCampo(d, 0, params, int32(38), false); err != nil {
		t.Fatalf("escribir height: %v", err)
	}
	if alto, _ := m.leeCampo(d, 0, params, false); alto != int32(38) {
		t.Errorf("el alto escrito es %v y se esperaba 38", alto)
	}
}

// TestLayoutParamsPuestosPorLaApp comprueba que un `setLayoutParams` previo gana sobre el objeto
// sintético: los parámetros que la app creó se devuelven tal cual, sin aviso de aproximación.
func TestLayoutParamsPuestosPorLaApp(t *testing.T) {
	m := maquinaDeStubs()
	vista := objDe("Landroidx/viewpager/widget/ViewPager;")
	puestos := objDe("Landroid/widget/LinearLayout$LayoutParams;")
	protoPoner := "(Landroid/view/ViewGroup$LayoutParams;)V"
	if _, ok, err := m.intentaStub([]string{"Landroid/view/View;"}, "setLayoutParams", protoPoner,
		[]any{puestos}, vista); err != nil || !ok {
		t.Fatalf("setLayoutParams: ok=%v err=%v", ok, err)
	}
	leidos, ok, err := m.intentaStub([]string{"Landroid/view/View;"}, "getLayoutParams",
		"()Landroid/view/ViewGroup$LayoutParams;", nil, vista)
	if err != nil || !ok || leidos != puestos {
		t.Fatalf("getLayoutParams no devolvió los parámetros puestos: %v %v %v", leidos, ok, err)
	}
	if len(m.avisos) != 0 {
		t.Errorf("no debería haber aviso de aproximación: %v", m.avisos)
	}
}

// TestBindServiceInerteParaTermux falsifica la frontera de §377: sin stub, `bindService` devolvía
// el cero de Z y `TermuxActivity.onCreate` lanzaba «bindService() failed». Con el stub devuelve
// verdadero, registra la conexión y deja un aviso de aproximación; `unbindService` la olvida.
func TestBindServiceInerteParaTermux(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	intento := objDe("Landroid/content/Intent;")
	conexion := actividad // Termux se entrega a sí mismo: implementa ServiceConnection.

	candidatos := []string{"Lcom/termux/app/TermuxActivity;", "Landroid/app/Activity;"}
	v, ok, err := m.intentaStub(candidatos, "bindService",
		"(Landroid/content/Intent;Landroid/content/ServiceConnection;I)Z",
		[]any{intento, conexion, int32(1)}, actividad)
	if err != nil || !ok {
		t.Fatalf("bindService: ok=%v err=%v", ok, err)
	}
	if v != int32(1) {
		t.Errorf("bindService devolvió %v y se esperaba verdadero", v)
	}
	if len(m.conexiones) != 1 || m.conexiones[0] != conexion {
		t.Fatalf("la conexión no quedó registrada: %v", m.conexiones)
	}
	if _, ligada := conexion.campos[conexionLigada]; !ligada {
		t.Error("la conexión registrada no lleva la marca de ligada")
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "bindService-inerte") {
		t.Errorf("falta el aviso de aproximación: %v", m.avisos)
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("bindService cayó en el aviso de stub ausente: %v", m.avisos)
	}

	// Desligar no falla y limpia el registro; no hay servicio real que detener.
	v, ok, err = m.intentaStub(candidatos, "unbindService",
		"(Landroid/content/ServiceConnection;)V", []any{conexion}, actividad)
	if err != nil || !ok || v != nil {
		t.Fatalf("unbindService: v=%v ok=%v err=%v", v, ok, err)
	}
	if len(m.conexiones) != 0 {
		t.Errorf("unbindService no olvidó la conexión: %v", m.conexiones)
	}
	if _, ligada := conexion.campos[conexionLigada]; ligada {
		t.Error("unbindService no quitó la marca de ligada")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "unbindService-inerte") {
		t.Errorf("falta el aviso de desligado inerte: %v", m.avisos)
	}
}

// TestCallbacksDeServiceConnectionInertes comprueba que los dos callbacks se atienden sin servicio
// detrás, tanto sobre un ServiceConnection del framework como sobre uno registrado por bindService.
func TestCallbacksDeServiceConnectionInertes(t *testing.T) {
	m := maquinaDeStubs()
	conexion := objDe("Landroid/content/ServiceConnection;")
	componente := objDe("Landroid/content/ComponentName;")
	binder := objDe("Landroid/os/IBinder;")

	casos := []struct {
		nombre string
		proto  string
		vals   []any
	}{
		{"onServiceConnected", "(Landroid/content/ComponentName;Landroid/os/IBinder;)V",
			[]any{componente, binder}},
		{"onServiceDisconnected", "(Landroid/content/ComponentName;)V", []any{componente}},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub([]string{"Landroid/content/ServiceConnection;"}, c.nombre,
			c.proto, c.vals, conexion)
		if err != nil || !ok || v != nil {
			t.Errorf("%s: v=%v ok=%v err=%v", c.nombre, v, ok, err)
		}
	}
	// La conexión ligada por bindService se reconoce por la marca, no solo por su clase.
	m.registraConexion(conexion)
	if v, ok, err := m.intentaStub([]string{"Lcom/termux/app/TermuxActivity;"},
		"onServiceDisconnected", "(Landroid/content/ComponentName;)V", []any{componente}, conexion); err != nil || !ok || v != nil {
		t.Errorf("onServiceDisconnected sobre la conexión ligada: v=%v ok=%v err=%v", v, ok, err)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "onServiceConnected-inerte") ||
		!strings.Contains(avisos, "onServiceDisconnected-inerte") {
		t.Errorf("faltan los avisos de los callbacks: %v", m.avisos)
	}
}

// TestLifecycleSubstringRotoSoloEnElDexDanado congela la discriminación que evita la regresión:
// el desvío al stub sólo se activa cuando el method_id de `String.substring(I)` trae el nombre
// corrupto. Con el nombre íntegro el cuerpo de Lifecycling se ejecuta como antes.
func TestLifecycleSubstringRotoSoloEnElDexDanado(t *testing.T) {
	m := &maquinaDalvik{}
	proto := dexProto{shorty: "(I)Ljava/lang/String;", returnType: "Ljava/lang/String;", params: []string{"I"}}
	bien := &dexFile{methods: []dexMethod{{owner: "Ljava/lang/String;", name: "substring", proto: proto}}}
	if m.substringRoto(bien) {
		t.Error("un method_id íntegro no debe activar el desvío al stub")
	}
	roto := &dexFile{methods: []dexMethod{{owner: "Ljava/lang/String;",
		name: ".initPrototypeId(int id) did not initialize id=", proto: proto}}}
	if !m.substringRoto(roto) {
		t.Error("el method_id corrupto de NewPipe debe activar el desvío al stub")
	}
	if m.substringRoto(nil) {
		t.Error("sin dex no hay method_id corrupto")
	}
}

// TestLifecycleObserverSinteticoDeLaLambdaNoEsNulo comprueba el extremo que destapó la frontera de NewPipe:
// la lambda sintética `ComponentActivity$$ExternalSyntheticLambda3` del propio dex entrega un
// `Class` no nulo, y su nombre canónico es el descriptor convertido a puntos, no una cadena vacía.
func TestLifecycleObserverSinteticoDeLaLambdaNoEsNulo(t *testing.T) {
	m := maquinaDeStubs()
	observador := objDe("Landroidx/activity/ComponentActivity$$ExternalSyntheticLambda3;")
	valor, ok, err := m.intentaStub([]string{"Ljava/lang/Object;"}, "getClass",
		"()Ljava/lang/Class;", nil, observador)
	if err != nil || !ok {
		t.Fatalf("getClass del observador sintético: ok=%v err=%v", ok, err)
	}
	clase := esperaObjetoDeStub(t, valor, "Ljava/lang/Class;")
	if clase.campos["Ljava/lang/Class;.$tipo"] != observador.clase {
		t.Errorf("el Class no guardó el descriptor del observador: %v", clase.campos)
	}
	nombre, ok, err := m.intentaStub([]string{"Ljava/lang/Class;"}, "getCanonicalName",
		"()Ljava/lang/String;", nil, clase)
	if err != nil || !ok {
		t.Fatalf("getCanonicalName del observador: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, nombre, "androidx.activity.ComponentActivity$$ExternalSyntheticLambda3")
}

// TestReceptorNuloSigueCortandoCongela el guardián de `invoca`: aunque el observador se sintetice,
// un `invoke-virtual` con el receptor nulo debe seguir devolviendo el error de siempre y no
// rellenarse con un objeto inventado.
func TestLifecycleReceptorNuloSigueCortando(t *testing.T) {
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/lang/Object;",
		name:  "getClass",
		proto: dexProto{shorty: "()Ljava/lang/Class;", returnType: "Ljava/lang/Class;"},
	}}}
	m := &maquinaDalvik{vistos: map[string]bool{}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "Landroidx/lifecycle/Lifecycling;")
	if err == nil || !strings.Contains(err.Error(), "java.lang.Object.getClass sobre un objeto nulo") {
		t.Errorf("el receptor nulo no cortó con el error esperado: %v", err)
	}
}

// TestGetObserverConstructorTypeConsistente comprueba que el entero que devuelve el dex depende
// sólo del tipo sintético del observador: una lambda sin adaptador generado da 1 (reflexión), un
// adaptador generado da 2, y la decisión se anota como aproximación en vez de fingir ejecución.
func TestLifecycleGetObserverConstructorTypeConsistente(t *testing.T) {
	m := maquinaDeStubs()
	lambda := objDe("Ljava/lang/Class;")
	lambda.campos["Ljava/lang/Class;.$tipo"] = "Landroidx/activity/ComponentActivity$$ExternalSyntheticLambda3;"
	adapter := objDe("Ljava/lang/Class;")
	adapter.campos["Ljava/lang/Class;.$tipo"] = "Landroidx/activity/ComponentActivity_LifecycleAdapter;"

	for _, caso := range []struct {
		clase  *objetoDalvik
		espera int32
		tipo   string
	}{
		{lambda, 1, "LifecycleObserver por reflexión"},
		{adapter, 2, "GeneratedAdapter"},
	} {
		for vez := 0; vez < 2; vez++ {
			v, ok, err := m.intentaStub([]string{"Landroidx/lifecycle/Lifecycling;"},
				"getObserverConstructorType", "(Ljava/lang/Class;)I", []any{caso.clase}, nil)
			if err != nil || !ok {
				t.Fatalf("getObserverConstructorType: ok=%v err=%v", ok, err)
			}
			if v != caso.espera {
				t.Errorf("devolvió %v y se esperaba %d para %s", v, caso.espera, caso.tipo)
			}
		}
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "sin ejecutar el cuerpo del dex (aproximación)") {
		t.Errorf("falta el aviso de aproximación: %s", avisos)
	}
	if !strings.Contains(avisos, "LifecycleObserver por reflexión") || !strings.Contains(avisos, "GeneratedAdapter") {
		t.Errorf("el aviso no distingue el tipo sintético: %s", avisos)
	}
}

// TestBroadcastReceiversListaVaciaNoNula comprueba que el PackageManager devuelve una lista
// no nula aunque no haya ningún receptor registrado, y lo hace con el aviso de aproximación.
func TestBroadcastReceiversListaVaciaNoNula(t *testing.T) {
	m := maquinaDeStubs()
	pm := objDe("Landroid/content/pm/PackageManager;")
	intento := objDe("Landroid/content/Intent;")
	lista, ok, err := m.intentaStub([]string{"Landroid/content/pm/PackageManager;"},
		"queryBroadcastReceivers", "(Landroid/content/Intent;I)Ljava/util/List;",
		[]any{intento, int32(0)}, pm)
	if err != nil || !ok || lista == nil {
		t.Fatalf("queryBroadcastReceivers: v=%v ok=%v err=%v", lista, ok, err)
	}
	l, ok := lista.(*objetoDalvik)
	if !ok || l.clase != "Ljava/util/ArrayList;" || l.lista == nil || len(l.lista) != 0 {
		t.Fatalf("la lista devuelta no es un ArrayList vacío: %#v", lista)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "queryBroadcastReceivers-inerte") {
		t.Errorf("falta el aviso de aproximación: %v", m.avisos)
	}
}

// TestBroadcastReceiversIteradorVacioNoNula comprueba la cadena que usaba Termux: el
// `iterator()` de la lista devuelta existe y su `hasNext()` da cero, sin un receptor nulo.
func TestBroadcastReceiversIteradorVacioNoNula(t *testing.T) {
	m := maquinaDeStubs()
	pm := objDe("Landroid/content/pm/PackageManager;")
	lista, ok, err := m.intentaStub([]string{"Landroid/content/pm/PackageManager;"},
		"queryBroadcastReceivers", "(Landroid/content/Intent;I)Ljava/util/List;",
		[]any{objDe("Landroid/content/Intent;"), int32(0)}, pm)
	if err != nil || !ok || lista == nil {
		t.Fatalf("queryBroadcastReceivers: v=%v ok=%v err=%v", lista, ok, err)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/List;"}, "iterator",
		"()Ljava/util/Iterator;", nil, lista)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator: v=%v ok=%v err=%v", it, ok, err)
	}
	v, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok {
		t.Fatalf("hasNext: ok=%v err=%v", ok, err)
	}
	if v != int32(0) {
		t.Errorf("hasNext devolvió %v y la lista está vacía", v)
	}
}

// TestBroadcastFalsificacionReceptorNulo reproduce la frontera anterior con los
// stubs apagados: `queryBroadcastReceivers` no se resuelve, el cero del List es un receptor nulo y
// el `iterator()` siguiente tampoco casa con ningún stub. Es el estado que hacía cortar a
// `sendTermuxOpenedBroadcast` con «se llamó a java.util.List.iterator sobre un objeto nulo».
func TestBroadcastFalsificacionReceptorNulo(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	m := maquinaDeStubs()
	pm := objDe("Landroid/content/pm/PackageManager;")
	v, ok, err := m.intentaStub([]string{"Landroid/content/pm/PackageManager;"},
		"queryBroadcastReceivers", "(Landroid/content/Intent;I)Ljava/util/List;",
		[]any{objDe("Landroid/content/Intent;"), int32(0)}, pm)
	if err != nil || ok || v != nil {
		t.Fatalf("con los stubs apagados queryBroadcastReceivers se resolvió: v=%v ok=%v err=%v",
			v, ok, err)
	}
	receptor := valorCero("Ljava/util/List;")
	if receptor != nil {
		t.Fatalf("el cero del tipo de retorno es %v y se esperaba el receptor nulo", receptor)
	}
	v, ok, err = m.intentaStub([]string{"Ljava/util/List;"}, "iterator", "()Ljava/util/Iterator;",
		nil, receptor)
	if err != nil || ok {
		t.Fatalf("iterator sobre el cero se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
}

// cadenaGetLifecycle ejecuta `getLifecycle` sobre el receptor y encadena `addObserver` sobre lo que
// devolvió, con un dex mínimo sin cuerpos para que la resolución pase por `intentaStub`. Es la
// misma cadena que cortaba en NewPipe, sin APK ni navegador.
func cadenaGetLifecycle(receptor any) (*maquinaDalvik, any, error) {
	d := &dexFile{methods: []dexMethod{
		{owner: "Landroidx/lifecycle/LifecycleOwner;", name: "getLifecycle",
			proto: dexProto{shorty: "LLifecycle;", returnType: "Landroidx/lifecycle/Lifecycle;"}},
		{owner: "Landroidx/lifecycle/Lifecycle;", name: "addObserver",
			proto: dexProto{shorty: "VL", returnType: "V", params: []string{"Ljava/lang/Object;"}}},
	}}
	m := nuevaMaquina(&programaDalvik{dex: d, clases: map[string]*claseDalvik{}}, nil)
	regs := []any{receptor, nil, objDe("Landroidx/lifecycle/DefaultLifecycleObserver;")}
	lifecycle, err := m.invoca(0x6e, d, 0, []int{0}, regs, "Lorg/schabi/newpipe/MainActivity;")
	if err != nil {
		return m, lifecycle, err
	}
	regs[1] = lifecycle
	_, err = m.invoca(0x6e, d, 1, []int{1, 2}, regs, "Lorg/schabi/newpipe/MainActivity;")
	return m, lifecycle, err
}

// TestGetLifecycleDevuelveElRegistryDelDex comprueba que el stub entrega la MISMA referencia que el
// dex guardó en el campo de la actividad: no se fabrica un LifecycleRegistry ni un observador.
func TestGetLifecycleDevuelveElRegistryDelDex(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lorg/schabi/newpipe/MainActivity;")
	registry := objDe("Landroidx/lifecycle/LifecycleRegistry;")
	actividad.campos["Landroidx/core/app/ComponentActivity;.lifecycleRegistry"] = registry

	for _, candidato := range []string{
		"Landroidx/lifecycle/LifecycleOwner;",
		"Landroidx/savedstate/SavedStateRegistryOwner;",
	} {
		v, ok, err := m.intentaStub([]string{candidato}, "getLifecycle",
			"()Landroidx/lifecycle/Lifecycle;", nil, actividad)
		if err != nil || !ok {
			t.Fatalf("getLifecycle con %s: ok=%v err=%v", candidato, ok, err)
		}
		if v != registry {
			t.Errorf("getLifecycle con %s devolvió %v y se esperaba el registry del dex", candidato, v)
		}
	}
	// es un getter sin argumentos: con uno no debe casar
	if v, ok, err := m.intentaStub([]string{"Landroidx/lifecycle/LifecycleOwner;"}, "getLifecycle",
		"()Landroidx/lifecycle/Lifecycle;", []any{objDe("Ljava/lang/Object;")}, actividad); err != nil || ok || v != nil {
		t.Errorf("getLifecycle con un argumento casó: v=%v ok=%v err=%v", v, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "LifecycleRegistry que el dex construyó") {
		t.Errorf("falta el aviso de aproximación: %s", avisos)
	}
}

// TestGetLifecycleLeeLasClavesDelDexDeGodot comprueba las dos claves `mLifecycleRegistry` que
// `dexdump` muestra en el dex de Godot: la de androidx.core y la de androidx.activity. Con una
// sola de ellas presente, `getLifecycle` entrega ESA referencia y el `addObserver` encadenado no
// corta. Es la frontera de 7279 pasos: con solo la clave histórica el cero llegaba al observador.
func TestGetLifecycleLeeLasClavesDelDexDeGodot(t *testing.T) {
	for _, clave := range []string{
		"Landroidx/core/app/ComponentActivity;.mLifecycleRegistry",
		"Landroidx/activity/ComponentActivity;.mLifecycleRegistry",
	} {
		t.Run(clave, func(t *testing.T) {
			actividad := objDe("Lcom/godot/game/GodotApp;")
			registry := objDe("Landroidx/lifecycle/LifecycleRegistry;")
			actividad.campos[clave] = registry

			for _, candidato := range []string{
				"Landroidx/lifecycle/LifecycleOwner;",
				"Landroidx/savedstate/SavedStateRegistryOwner;",
			} {
				m := maquinaDeStubs()
				v, ok, err := m.intentaStub([]string{candidato}, "getLifecycle",
					"()Landroidx/lifecycle/Lifecycle;", nil, actividad)
				if err != nil || !ok {
					t.Fatalf("getLifecycle con %s y clave %s: ok=%v err=%v", candidato, clave, ok, err)
				}
				if v != registry {
					t.Errorf("getLifecycle con %s y clave %s devolvió %v y se esperaba el registry del dex",
						candidato, clave, v)
				}
				if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "LifecycleRegistry que el dex construyó") {
					t.Errorf("falta el aviso de aproximación con %s: %s", clave, avisos)
				}
			}

			_, lifecycle, err := cadenaGetLifecycle(actividad)
			if lifecycle != registry {
				t.Fatalf("cadena con clave %s devolvió %v y se esperaba el registry", clave, lifecycle)
			}
			if err != nil {
				t.Fatalf("addObserver con clave %s falló: %v", clave, err)
			}
		})
	}
}

// TestGetLifecycleConStubsApagadosReapareceElError falsifica la base con el interruptor de
// previewstubsframework_test.go:232. Sin el stub, el cero del tipo llega a `addObserver` y reaparece
// «se llamó a androidx.lifecycle.Lifecycle.addObserver sobre un objeto nulo».
func TestGetLifecycleConStubsApagadosReapareceElError(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	actividad := objDe("Lorg/schabi/newpipe/MainActivity;")
	registry := objDe("Landroidx/lifecycle/LifecycleRegistry;")
	actividad.campos["Landroidx/core/app/ComponentActivity;.lifecycleRegistry"] = registry

	_, lifecycle, err := cadenaGetLifecycle(actividad)
	if lifecycle != nil {
		t.Fatalf("con los stubs apagados getLifecycle devolvió %v y se esperaba el cero", lifecycle)
	}
	if err == nil || !strings.Contains(err.Error(),
		"androidx.lifecycle.Lifecycle.addObserver sobre un objeto nulo") {
		t.Fatalf("no reapareció el error original: %v", err)
	}

	// con los stubs encendidos la misma cadena entrega el registry y no corta
	stubsFrameworkActivos = true
	_, lifecycle, err = cadenaGetLifecycle(actividad)
	if lifecycle != registry {
		t.Fatalf("con los stubs encendidos getLifecycle devolvió %v y se esperaba el registry", lifecycle)
	}
	if err != nil {
		t.Fatalf("addObserver sobre el registry falló: %v", err)
	}
}

// TestGetLifecycleReceptorSinCampoSigueCortando impide que el stub invente un lifecycle: sin el
// campo que el propio dex escribe no casa, se devuelve el cero y `addObserver` corta igual que antes.
func TestGetLifecycleReceptorSinCampoSigueCortando(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lorg/schabi/newpipe/MainActivity;")
	v, ok, err := m.intentaStub([]string{"Landroidx/lifecycle/LifecycleOwner;"}, "getLifecycle",
		"()Landroidx/lifecycle/Lifecycle;", nil, actividad)
	if err != nil || ok || v != nil {
		t.Fatalf("sin el campo getLifecycle se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}

	m, lifecycle, err := cadenaGetLifecycle(actividad)
	if lifecycle != nil {
		t.Fatalf("sin el campo getLifecycle devolvió %v y se esperaba el cero", lifecycle)
	}
	if err == nil || !strings.Contains(err.Error(), "Lifecycle.addObserver sobre un objeto nulo") {
		t.Fatalf("sin el campo no reapareció el error original: %v", err)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("el cero no se anunció como aproximación: %s", avisos)
	}
}

// TestDatosDelMapaAceptaLinkedHashMap fija el respaldo del `keyToProviders` que el dex crea con
// `new-instance Ljava/util/LinkedHashMap;`: sin él, `entrySet` devolvía el cero de Ljava/util/Set;
// y el `Set.iterator` siguiente reventaba en F-Droid y NewPipe. El mapa nace vacío: no se inventa
// ninguna clave.
func TestDatosDelMapaAceptaLinkedHashMap(t *testing.T) {
	mapa := objDe("Ljava/util/LinkedHashMap;")
	// la clave se siembra antes de la primera lectura: datosDelMapa cachea el respaldo en
	// `mapa.datos` y una lectura posterior no recorre otra vez los campos `mapa.*`.
	mapa.campos["mapa.claveSembrada"] = cadenaObjeto("valorSembrado")
	datos := datosDelMapa(mapa)
	if datos == nil {
		t.Fatal("datosDelMapa no reconoció el LinkedHashMap del dex")
	}
	if textoDe(datos["claveSembrada"]) != "valorSembrado" {
		t.Errorf("la clave sembrada no se leyó: %v", datos)
	}
	// un mapa sin sembrar nace vacío y no inventa ninguna clave
	if datos := datosDelMapa(objDe("Ljava/util/LinkedHashMap;")); len(datos) != 0 {
		t.Errorf("el mapa nació con %d claves inventadas", len(datos))
	}
}

// TestGetSavedStateRegistryDevuelveElDelDex comprueba que el getter acotado entrega solo la
// referencia que el dex guardó en el controlador de la actividad, y que sin ese campo no casa.
func TestGetSavedStateRegistryDevuelveElDelDex(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lorg/schabi/newpipe/MainActivity;")
	registry := objDe("Landroidx/savedstate/SavedStateRegistry;")
	controlador := objDe("Landroidx/savedstate/SavedStateRegistryController;")
	controlador.campos["Landroidx/savedstate/SavedStateRegistryController;.savedStateRegistry"] = registry
	actividad.campos["Landroidx/activity/ComponentActivity;.mSavedStateRegistryController"] = controlador

	v, ok, err := m.intentaStub([]string{"Landroidx/savedstate/SavedStateRegistryOwner;"},
		"getSavedStateRegistry", "()Landroidx/savedstate/SavedStateRegistry;", nil, actividad)
	if err != nil || !ok || v != registry {
		t.Fatalf("getSavedStateRegistry: v=%v ok=%v err=%v", v, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "SavedStateRegistry que el dex construyó") {
		t.Errorf("falta el aviso de aproximación: %s", avisos)
	}

	// otra firma y otro receptor no deben casa: el stub no inventa un registry
	if v, ok, err := m.intentaStub([]string{"Landroidx/savedstate/SavedStateRegistryOwner;"},
		"getSavedStateRegistry", "()Landroidx/savedstate/SavedStateRegistry;", nil,
		objDe("Lorg/schabi/newpipe/MainActivity;")); err != nil || ok || v != nil {
		t.Errorf("sin el campo getSavedStateRegistry se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
}

// TestGetSavedStateRegistryConStubsApagados no entrega la referencia: el interruptor de
// previewstubsframework_test.go:232 debe devolverla al aviso de getter ausente de antes.
func TestGetSavedStateRegistryConStubsApagados(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()
	m := maquinaDeStubs()
	actividad := objDe("Lorg/schabi/newpipe/MainActivity;")
	registry := objDe("Landroidx/savedstate/SavedStateRegistry;")
	controlador := objDe("Landroidx/savedstate/SavedStateRegistryController;")
	controlador.campos["Landroidx/savedstate/SavedStateRegistryController;.savedStateRegistry"] = registry
	actividad.campos["Landroidx/activity/ComponentActivity;.mSavedStateRegistryController"] = controlador

	v, ok, err := m.intentaStub([]string{"Landroidx/savedstate/SavedStateRegistryOwner;"},
		"getSavedStateRegistry", "()Landroidx/savedstate/SavedStateRegistry;", nil, actividad)
	if err != nil || ok || v != nil {
		t.Fatalf("con los stubs apagados getSavedStateRegistry se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
}

// cadenaFragmentSavedState ejecuta el par de llamadas que el `FragmentActivity.init` de Godot hace
// en el dex: el getter con `FragmentActivity` como dueño del ref y el
// `registerSavedStateProvider` sobre lo que devolvió. El dex es mínimo y no tiene cuerpos, así que
// la resolución pasa por `intentaStub`, igual que en la APK real.
func cadenaFragmentSavedState(receptor any) (*maquinaDalvik, any, error) {
	d := &dexFile{methods: []dexMethod{
		{owner: "Landroidx/fragment/app/FragmentActivity;", name: "getSavedStateRegistry",
			proto: dexProto{shorty: "LSavedStateRegistry;", returnType: "Landroidx/savedstate/SavedStateRegistry;"}},
		{owner: "Landroidx/savedstate/SavedStateRegistry;", name: "registerSavedStateProvider",
			proto: dexProto{shorty: "VLL", returnType: "V",
				params: []string{"Ljava/lang/String;", "Landroidx/savedstate/SavedStateRegistry$SavedStateProvider;"}}},
	}}
	m := nuevaMaquina(&programaDalvik{dex: d, clases: map[string]*claseDalvik{}}, nil)
	regs := []any{receptor, nil, cadenaObjeto("clave"),
		objDe("Landroidx/savedstate/SavedStateRegistry$SavedStateProvider;")}
	registry, err := m.invoca(0x6e, d, 0, []int{0}, regs, "Lorg/godotengine/godot/GodotActivity;")
	if err != nil {
		return m, registry, err
	}
	regs[1] = registry
	_, err = m.invoca(0x6e, d, 1, []int{1, 2, 3}, regs, "Lorg/godotengine/godot/GodotActivity;")
	return m, registry, err
}

// actividadConControladorDeSavedState monta la actividad con el controlador que el propio dex crea;
// es la única fuente del registry, sin objetos fabricados por el stub.
func actividadConControladorDeSavedState() (*objetoDalvik, *objetoDalvik) {
	actividad := objDe("Lorg/godotengine/godot/GodotActivity;")
	registry := objDe("Landroidx/savedstate/SavedStateRegistry;")
	controlador := objDe("Landroidx/savedstate/SavedStateRegistryController;")
	controlador.campos["Landroidx/savedstate/SavedStateRegistryController;.savedStateRegistry"] = registry
	actividad.campos["Landroidx/activity/ComponentActivity;.mSavedStateRegistryController"] = controlador
	return actividad, registry
}

// TestFragmentSavedStateDevuelveElRegistryDelControlador falsifica el caso de Godot: el getter
// declarado en FragmentActivity entrega la MISMA referencia que el controlador de la actividad
// guardó, avisa de la aproximación y el `registerSavedStateProvider` siguiente no corta.
func TestFragmentSavedStateDevuelveElRegistryDelControlador(t *testing.T) {
	actividad, registry := actividadConControladorDeSavedState()
	m := maquinaDeStubs()
	v, ok, err := m.intentaStub([]string{"Landroidx/fragment/app/FragmentActivity;"},
		"getSavedStateRegistry", "()Landroidx/savedstate/SavedStateRegistry;", nil, actividad)
	if err != nil || !ok || v != registry {
		t.Fatalf("FragmentActivity.getSavedStateRegistry: v=%v ok=%v err=%v", v, ok, err)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "FragmentActivity.getSavedStateRegistry") ||
		!strings.Contains(avisos, "aproximación") {
		t.Errorf("falta el aviso de aproximación de FragmentActivity: %s", avisos)
	}

	m, obtenido, err := cadenaFragmentSavedState(actividad)
	if err != nil {
		t.Fatalf("la cadena de Godot cortó con el registry del controlador: %v", err)
	}
	if obtenido != registry {
		t.Errorf("la cadena devolvió %v y se esperaba el registry del controlador", obtenido)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos,
		"FragmentActivity.getSavedStateRegistry()Landroidx/savedstate/SavedStateRegistry; no está en el dex ni en los stubs") {
		t.Errorf("sigue el aviso del cero de antes: %s", avisos)
	}
}

// TestFragmentSavedStateSinControladorSigueCortando impide que el stub invente un registry: sin el
// controlador que escribe el dex, el cero llega al `registerSavedStateProvider` y corta igual.
func TestFragmentSavedStateSinControladorSigueCortando(t *testing.T) {
	actividad := objDe("Lorg/godotengine/godot/GodotActivity;")
	m, registry, err := cadenaFragmentSavedState(actividad)
	if registry != nil {
		t.Fatalf("sin controlador se devolvió %v y se esperaba el cero", registry)
	}
	if err == nil || !strings.Contains(err.Error(),
		"SavedStateRegistry.registerSavedStateProvider sobre un objeto nulo") {
		t.Fatalf("sin controlador no reapareció la frontera original: %v", err)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"FragmentActivity.getSavedStateRegistry()Landroidx/savedstate/SavedStateRegistry; no está en el dex ni en los stubs") {
		t.Errorf("el cero no se anunció como antes: %s", avisos)
	}
}

// TestFragmentSavedStateConStubsApagadosReapareceElError falsifica el caso A con el interruptor de
// previewstubsframework_test.go:232: sin el stub, aunque la actividad tenga controlador, reaparece
// la frontera de 8843 pasos; con él la misma cadena sigue.
func TestFragmentSavedStateConStubsApagadosReapareceElError(t *testing.T) {
	actividad, registry := actividadConControladorDeSavedState()
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	_, obtenido, err := cadenaFragmentSavedState(actividad)
	if obtenido != nil {
		t.Fatalf("con los stubs apagados se devolvió %v y se esperaba el cero", obtenido)
	}
	if err == nil || !strings.Contains(err.Error(),
		"SavedStateRegistry.registerSavedStateProvider sobre un objeto nulo") {
		t.Fatalf("con los stubs apagados no reapareció el error original: %v", err)
	}

	stubsFrameworkActivos = true
	_, obtenido, err = cadenaFragmentSavedState(actividad)
	if err != nil {
		t.Fatalf("con los stubs encendidos la cadena cortó: %v", err)
	}
	if obtenido != registry {
		t.Fatalf("con los stubs encendidos se devolvió %v y se esperaba el registry", obtenido)
	}
}

// aíslaServicios deja el registro de servicios vacío durante una prueba y lo restaura al salir. El
// interruptor `cargados` se pone a true para que la prueba no abra la APK del hospedador: todo lo que
// vea el iterador tiene que venir del zip que siembra la propia prueba.
func aíslaServicios(t *testing.T) {
	t.Helper()
	anterior := serviciosDelApk
	cargados := serviciosDelApkCargados
	t.Cleanup(func() {
		serviciosDelApk = anterior
		serviciosDelApkCargados = cargados
	})
	serviciosDelApk = map[string][]string{}
	serviciosDelApkCargados = true
}

// zipConServicio arma un APK mínimo en memoria con una sola entrada META-INF/services/<interfaz>.
func zipConServicio(t *testing.T, interfaz, contenido string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("META-INF/services/" + interfaz)
	if err != nil {
		t.Fatalf("no se pudo crear la entrada de servicios: %v", err)
	}
	if _, err := w.Write([]byte(contenido)); err != nil {
		t.Fatalf("no se pudo escribir la entrada de servicios: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("no se pudo cerrar el zip de servicios: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("el zip de servicios no se pudo leer: %v", err)
	}
	return zr
}

// claseParaServicio es el `java.lang.Class` que el intérprete guarda para un literal `X.class`.
func claseParaServicio(descriptor string) *objetoDalvik {
	return &objetoDalvik{clase: "Ljava/lang/Class;", campos: map[string]any{
		"Ljava/lang/Class;.$tipo": descriptor,
	}}
}

// cargadorDeServicio pide el stub de ServiceLoader.load con la firma que F-Droid no encuentra en el
// dex y devuelve el objeto no nulo.
func cargadorDeServicio(t *testing.T, m *maquinaDalvik, descriptor string) *objetoDalvik {
	t.Helper()
	const proto = "(Ljava/lang/Class;Ljava/lang/ClassLoader;)Ljava/util/ServiceLoader;"
	v, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "load", proto,
		[]any{claseParaServicio(descriptor), nil}, nil)
	if err != nil || !ok {
		t.Fatalf("ServiceLoader.load(%s): ok=%v err=%v", descriptor, ok, err)
	}
	return esperaObjetoDeStub(t, v, "Ljava/util/ServiceLoader;")
}

// TestServiceLoaderLeeElServicioRealDelApk falsifica que el iterador entrega UNICAMENTE el nombre
// escrito en META-INF/services del APK, con su comentario y sus líneas vacías ya descartados, y que
// el aviso dice que se entregan nombres en vez de instancias.
func TestServiceLoaderLeeElServicioRealDelApk(t *testing.T) {
	aíslaServicios(t)
	registraServiciosDelApk(zipConServicio(t, "org.slf4j.spi.SLF4JServiceProvider",
		"# proveedor declarado en el APK\norg.slf4j.impl.LoggerServiceProvider\n\n"))
	m := maquinaDeStubs()
	loader := cargadorDeServicio(t, m, "Lorg/slf4j/spi/SLF4JServiceProvider;")
	if loader.campos[markerServicioCargador] != "org.slf4j.spi.SLF4JServiceProvider" {
		t.Errorf("el cargador no quedó ligado a la interfaz: %v", loader.campos)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "iterator",
		"()Ljava/util/Iterator;", nil, loader)
	if err != nil || !ok {
		t.Fatalf("ServiceLoader.iterator: ok=%v err=%v", ok, err)
	}
	cursor := esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
	if len(cursor.iterando) != 1 {
		t.Fatalf("el iterador trae %d proveedores y el fichero declaraba uno: %v", len(cursor.iterando), cursor.iterando)
	}
	mas, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, cursor)
	if err != nil || !ok || mas != int32(1) {
		t.Fatalf("hasNext: v=%v ok=%v err=%v", mas, ok, err)
	}
	siguiente, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "next",
		"()Ljava/lang/Object;", nil, cursor)
	if err != nil || !ok {
		t.Fatalf("next: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, siguiente, "org.slf4j.impl.LoggerServiceProvider")
	mas, _, _ = m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, cursor)
	if mas != int32(0) {
		t.Errorf("el iterador no se agotó tras next: %v", mas)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "aproximación") {
		t.Errorf("falta el aviso de aproximación: %s", avisos)
	}
}

// TestServiceLoaderSinFicheroDaIteradorVacio cubre las dos llamadas de coil3: una interfaz sin
// entrada META-INF/services entrega un iterador no nulo y vacío, sin fabricar proveedores.
func TestServiceLoaderSinFicheroDaIteradorVacio(t *testing.T) {
	aíslaServicios(t)
	m := maquinaDeStubs()
	for _, descriptor := range []string{"Lcoil3/ImageLoader;", "Lcoil3/ImageLoaderFactory;"} {
		loader := cargadorDeServicio(t, m, descriptor)
		it, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "iterator",
			"()Ljava/util/Iterator;", nil, loader)
		if err != nil || !ok {
			t.Fatalf("iterator de %s: ok=%v err=%v", descriptor, ok, err)
		}
		cursor := esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
		if len(cursor.iterando) != 0 {
			t.Errorf("%s entregó %d proveedores sin fichero: %v", descriptor, len(cursor.iterando), cursor.iterando)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "0 proveedor(es)") {
		t.Errorf("el aviso no dice que no hay proveedores: %s", avisos)
	}
}

// TestServiceLoaderReceptorNuloSigueCortando congela el guardián de `invoca`: aunque el stub devuelva
// un cargador, un iterator invocado sobre un receptor nulo debe seguir dando el error original.
func TestServiceLoaderReceptorNuloSigueCortando(t *testing.T) {
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/util/ServiceLoader;",
		name:  "iterator",
		proto: dexProto{shorty: "()Ljava/util/Iterator;", returnType: "Ljava/util/Iterator;"},
	}}}
	m := &maquinaDalvik{vistos: map[string]bool{}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "Ljava/util/ServiceLoader;")
	if err == nil || !strings.Contains(err.Error(), "java.util.ServiceLoader.iterator sobre un objeto nulo") {
		t.Errorf("el receptor nulo no cortó con el error esperado: %v", err)
	}
}

// TestServiceLoaderConStubsApagadosVuelveAlCero es la falsación A/B: con el interruptor apagado el
// cargador no se modela, el cero llega al iterador y reaparece la frontera de 7272 pasos.
func TestServiceLoaderConStubsApagadosVuelveAlCero(t *testing.T) {
	aíslaServicios(t)
	registraServiciosDelApk(zipConServicio(t, "org.slf4j.spi.SLF4JServiceProvider",
		"org.slf4j.impl.LoggerServiceProvider\n"))
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()
	m := maquinaDeStubs()
	const proto = "(Ljava/lang/Class;Ljava/lang/ClassLoader;)Ljava/util/ServiceLoader;"
	v, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "load", proto,
		[]any{claseParaServicio("Lorg/slf4j/spi/SLF4JServiceProvider;"), nil}, nil)
	if err != nil || ok || v != nil {
		t.Errorf("con los stubs apagados load se resolvió: v=%v ok=%v err=%v", v, ok, err)
	}
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/util/ServiceLoader;",
		name:  "iterator",
		proto: dexProto{shorty: "()Ljava/util/Iterator;", returnType: "Ljava/util/Iterator;"},
	}}}
	_, err = m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "Ljava/util/ServiceLoader;")
	if err == nil || !strings.Contains(err.Error(), "java.util.ServiceLoader.iterator sobre un objeto nulo") {
		t.Errorf("sin el stub no reapareció el error original: %v", err)
	}
}

// apkServiciosMontada permite correr la falsificación en el hospedador de Node con la APK real
// montada: el shim no propaga variables de entorno al binario de Go, pero sí sus argumentos.
var apkServiciosMontada = flag.String("apk-servicios", "", "APK montada en el hospedador para la prueba de servicios")

// aíslaLosServiciosDelPreview limpia el registro global y lo devuelve al salir. El indicador se pone
// a false a propósito: la llamada de preview.go tiene que ser la que lo active desde el zip.
func aíslaLosServiciosDelPreview(t *testing.T) {
	t.Helper()
	anterior := serviciosDelApk
	cargados := serviciosDelApkCargados
	t.Cleanup(func() {
		serviciosDelApk = anterior
		serviciosDelApkCargados = cargados
	})
	serviciosDelApk = map[string][]string{}
	serviciosDelApkCargados = false
}

// apkConServicioParaPreview arma el paquete mínimo que la vista previa necesita: manifiesto escrito
// por el fork y la entrada META-INF/services con el proveedor que se quiera medir.
func apkConServicioParaPreview(t *testing.T, proveedor string) []byte {
	t.Helper()
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto de la prueba: %v", err)
	}
	return zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
		"META-INF/services/org.slf4j.spi.SLF4JServiceProvider": []byte(
			"# proveedor declarado en el APK\n" + proveedor + "\n\n"),
	})
}

// TestServiciosDelApkDisponiblesEnLaPestana falsifica el enganche de preview.go: con una APK señuelo
// en os.Args, el proveedor que entrega ServiceLoader viene del zip que recibió la vista previa y no
// del hospedador. Si se retira registraServiciosDeLaAPKEnPreview, el mapa queda vacío y falla.
func TestServiciosDelApkDisponiblesEnLaPestana(t *testing.T) {
	aíslaLosServiciosDelPreview(t)
	datos := apkConServicioParaPreview(t, "org.slf4j.impl.LoggerServiceProvider")
	if ruta := *apkServiciosMontada; ruta != "" {
		montada, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatalf("la APK montada %q no se pudo leer: %v", ruta, err)
		}
		datos = montada
		t.Logf("APK montada: %s (%d bytes)", ruta, len(datos))
	}
	senuelo := filepath.Join(t.TempDir(), "senuelo.apk")
	if err := os.WriteFile(senuelo, apkConServicioParaPreview(t, "org.fdroid.ProveedorInventado"), 0o600); err != nil {
		t.Fatalf("el APK señuelo no se pudo escribir: %v", err)
	}
	argsPrevios := os.Args
	os.Args = []string{"apkc", "preview", senuelo}
	t.Cleanup(func() { os.Args = argsPrevios })

	modelo, err := modeloVistaDeAPKCon(datos, "", false)
	if err != nil || modelo == nil {
		t.Fatalf("la vista previa de la APK con servicios falló: modelo=%v err=%v", modelo, err)
	}
	proveedores := serviciosDelApk["org.slf4j.spi.SLF4JServiceProvider"]
	if len(proveedores) != 1 || proveedores[0] != "org.slf4j.impl.LoggerServiceProvider" {
		t.Fatalf("el zip no aportó el proveedor real (o se leyó el señuelo): %v", proveedores)
	}

	m := maquinaDeStubs()
	loader := cargadorDeServicio(t, m, "Lorg/slf4j/spi/SLF4JServiceProvider;")
	it, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "iterator",
		"()Ljava/util/Iterator;", nil, loader)
	if err != nil || !ok {
		t.Fatalf("ServiceLoader.iterator: ok=%v err=%v", ok, err)
	}
	cursor := esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
	if len(cursor.iterando) != 1 {
		t.Fatalf("el iterador trae %d proveedores y el fichero declaraba uno: %v", len(cursor.iterando), cursor.iterando)
	}
	siguiente, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "next",
		"()Ljava/lang/Object;", nil, cursor)
	if err != nil || !ok {
		t.Fatalf("next: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, siguiente, "org.slf4j.impl.LoggerServiceProvider")
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "1 proveedor(es)") ||
		!strings.Contains(avisos, "aproximación") {
		t.Errorf("el aviso no dice cuántos proveedores se entregan ni que es una aproximación: %s", avisos)
	}
}

// TestServiciosDelApkSinAPKEnArgs falsifica que el proveedor llega solo del zip ya abierto: sin
// ninguna APK en los argumentos, la vista previa entrega el nombre escrito en META-INF/services.
func TestServiciosDelApkSinAPKEnArgs(t *testing.T) {
	aíslaLosServiciosDelPreview(t)
	datos := apkConServicioParaPreview(t, "org.slf4j.impl.LoggerServiceProvider")
	argsPrevios := os.Args
	os.Args = []string{"apkc", "preview"}
	t.Cleanup(func() { os.Args = argsPrevios })

	modelo, err := modeloVistaDeAPKCon(datos, "", false)
	if err != nil || modelo == nil {
		t.Fatalf("la vista previa sin APK en argumentos falló: modelo=%v err=%v", modelo, err)
	}
	proveedores := serviciosDelApk["org.slf4j.spi.SLF4JServiceProvider"]
	if len(proveedores) != 1 || proveedores[0] != "org.slf4j.impl.LoggerServiceProvider" {
		t.Fatalf("sin APK en os.Args el proveedor no vino del zip: %v", proveedores)
	}

	m := maquinaDeStubs()
	loader := cargadorDeServicio(t, m, "Lorg/slf4j/spi/SLF4JServiceProvider;")
	it, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "iterator",
		"()Ljava/util/Iterator;", nil, loader)
	if err != nil || !ok {
		t.Fatalf("ServiceLoader.iterator: ok=%v err=%v", ok, err)
	}
	cursor := esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
	if len(cursor.iterando) != 1 {
		t.Fatalf("el iterador trae %d proveedores y el fichero declaraba uno: %v", len(cursor.iterando), cursor.iterando)
	}
	siguiente, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "next",
		"()Ljava/lang/Object;", nil, cursor)
	if err != nil || !ok {
		t.Fatalf("next: ok=%v err=%v", ok, err)
	}
	esperaCadenaDeStub(t, siguiente, "org.slf4j.impl.LoggerServiceProvider")
}

// TestPreviewSinFicheroDeServiciosDaIteradorVacio comprueba el otro lado del enganche: una APK sin
// entrada META-INF/services deja el mapa vacío y el iterador no inventa ningún proveedor.
func TestPreviewSinFicheroDeServiciosDaIteradorVacio(t *testing.T) {
	aíslaLosServiciosDelPreview(t)
	manifiesto, err := escribeAXML(manifiestoDelStudio())
	if err != nil {
		t.Fatalf("el manifiesto de la prueba: %v", err)
	}
	argsPrevios := os.Args
	os.Args = []string{"apkc", "preview"}
	t.Cleanup(func() { os.Args = argsPrevios })
	modelo, err := modeloVistaDeAPKCon(zipDeEntradas(t, map[string][]byte{
		"AndroidManifest.xml": manifiesto,
	}), "", false)
	if err != nil || modelo == nil {
		t.Fatalf("la vista previa sin servicios falló: modelo=%v err=%v", modelo, err)
	}
	if len(serviciosDelApk) != 0 {
		t.Fatalf("sin entrada de servicios el registro no está vacío: %v", serviciosDelApk)
	}
	m := maquinaDeStubs()
	loader := cargadorDeServicio(t, m, "Lcoil3/ImageLoader;")
	it, ok, err := m.intentaStub([]string{"Ljava/util/ServiceLoader;"}, "iterator",
		"()Ljava/util/Iterator;", nil, loader)
	if err != nil || !ok {
		t.Fatalf("iterator sin fichero: ok=%v err=%v", ok, err)
	}
	cursor := esperaObjetoDeStub(t, it, "Ljava/util/Iterator;")
	if len(cursor.iterando) != 0 {
		t.Errorf("el iterador inventó %d proveedores sin fichero: %v", len(cursor.iterando), cursor.iterando)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "0 proveedor(es)") {
		t.Errorf("el aviso no dice que no hay proveedores: %s", avisos)
	}
}

// tablaDeTemaSplash escribe un color y un estilo con dos atributos reales: uno apunta al color y el
// otro a un atributo del framework ausente de la tabla. No hay aquí ningún valor inventado.
func tablaDeTemaSplash(t *testing.T) *tablaRecursos {
	t.Helper()
	tb := tablaEscribir{
		idPaquete:     0x7f,
		nombrePaquete: "com.prueba.tema",
		global:        []string{"fondo", "tema"},
		globalUTF8:    true,
		nombresTipo:   []string{"color", "style"},
		claves:        []string{"fondo", "tema"},
		clavesUTF8:    true,
		tipos: []tipoEscribir{
			{id: 1, specFlags: []uint32{0x00000001}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 0, valor: valorEscribir{tipo: tipoColorRGB8, datos: 0xff336699}},
			}}}},
			{id: 2, specFlags: []uint32{0x00000001}, bloques: []bloqueEscribir{{entradas: []entradaEscribir{
				{clave: 1, flags: 0x0001, compleja: true, registros: []registroEscribir{
					{nombre: 0x7f030015, valor: valorEscribir{tipo: tipoReferencia, datos: 0x7f010000}},
					{nombre: 0x7f030013, valor: valorEscribir{tipo: tipoAtributo, datos: 0x01010031}},
				}},
			}}}},
		},
	}
	datos, err := escribeTablaRecursos(tb)
	if err != nil {
		t.Fatalf("la tabla del tema: %v", err)
	}
	tr, err := leeTabla(datos)
	if err != nil {
		t.Fatalf("la tabla del tema no se deja leer: %v", err)
	}
	return tr
}

// TestTemaSplash falsifica el primer paso de la frontera de 8980 pasos: `getTheme` tiene que
// entregar un Theme no nulo enlazado a la actividad ya a la tabla del APK.
func TestTemaSplash(t *testing.T) {
	m := maquinaDeStubs()
	m.tabla = tablaDeTemaSplash(t)
	actividad := objDe("Lcom/prueba/GodotApp;")
	m.actividad = actividad

	v, ok, err := m.intentaStub([]string{"Lcom/prueba/GodotApp;", "Landroid/app/Activity;"},
		"getTheme", "()Landroid/content/res/Resources$Theme;", nil, actividad)
	if err != nil || !ok || v == nil {
		t.Fatalf("getTheme: v=%v ok=%v err=%v", v, ok, err)
	}
	tema := esperaObjetoDeStub(t, v, "Landroid/content/res/Resources$Theme;")
	if !esTemaSintetico(tema) || tablaDeTemaSintetico(tema) != m.tabla {
		t.Fatal("el Theme no está enlazado a la tabla del APK")
	}
	if tema.campos["tema.actividad"] != actividad {
		t.Fatal("el Theme no está enlazado a la actividad")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos, "tema-sintetico: Activity.getTheme") {
		t.Errorf("getTheme no dejó el aviso del tema sintético: %v", m.avisos)
	}
}

// TestResolveAttribute comprueba las dos mitades del contrato: un id de la tabla se resuelve
// escribiendo el TypedValue, y un id del framework ausente de resources.arsc devuelve false sin
// escribir un color inventado.
func TestResolveAttribute(t *testing.T) {
	m := maquinaDeStubs()
	m.tabla = tablaDeTemaSplash(t)
	tema := m.temaSintetico(objDe("Lcom/prueba/GodotApp;"))
	destino := objDe("Landroid/util/TypedValue;")
	candidatos := []string{"Landroid/content/res/Resources$Theme;"}

	v, ok, err := m.intentaStub(candidatos, "resolveAttribute", "(ILandroid/util/TypedValue;Z)Z",
		[]any{int32(0x7f030015), destino, int32(1)}, tema)
	if err != nil || !ok || v != int32(1) {
		t.Fatalf("resolveAttribute: v=%v ok=%v err=%v", v, ok, err)
	}
	if destino.campos["Landroid/util/TypedValue;.resourceId"] != int32(0x7f010000) {
		t.Errorf("resourceId=%v y se esperaba 0x7f010000", destino.campos["Landroid/util/TypedValue;.resourceId"])
	}
	colorEsperado := uint32(0xff336699)
	if destino.campos["Landroid/util/TypedValue;.data"] != int32(colorEsperado) {
		t.Errorf("data=%v y se esperaba %#08x", destino.campos["Landroid/util/TypedValue;.data"], colorEsperado)
	}

	otro := objDe("Landroid/util/TypedValue;")
	v, ok, err = m.intentaStub(candidatos, "resolveAttribute", "(ILandroid/util/TypedValue;Z)Z",
		[]any{int32(0x7f030013), otro, int32(1)}, tema)
	if err != nil || !ok || v != int32(0) {
		t.Fatalf("un attr del framework debería devolver false: v=%v ok=%v err=%v", v, ok, err)
	}
	if len(otro.campos) != 0 {
		t.Errorf("un id ausente escribió el TypedValue: %v", otro.campos)
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "resuelto desde style") ||
		!strings.Contains(avisos, "apunta a un id ausente de resources.arsc") {
		t.Errorf("faltaron los avisos de la tabla: %v", m.avisos)
	}
}

// TestViewTreeObserverNoNuloLigadoALaVista falsifica la frontera de los 7072 pasos: el dex de
// Termux llama a `getViewTreeObserver` sobre su `TermuxActivityRootView`, que hereda de
// LinearLayout, y el framework no trae cuerpo para ese método. El observador tiene que existir,
// pertenecer a esa vista y ser el mismo en la segunda llamada.
func TestViewTreeObserverNoNuloLigadoALaVista(t *testing.T) {
	m := maquinaDeStubs()
	raiz := "Lcom/termux/app/terminal/TermuxActivityRootView;"
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{
		raiz: {nombre: raiz, super: "Landroid/widget/LinearLayout;"},
	}}
	vista := objDe(raiz)
	proto := "()Landroid/view/ViewTreeObserver;"
	obs, ok, err := m.intentaStub([]string{raiz}, "getViewTreeObserver", proto, nil, vista)
	if err != nil || !ok {
		t.Fatalf("getViewTreeObserver: v=%v ok=%v err=%v", obs, ok, err)
	}
	observer, esObjeto := obs.(*objetoDalvik)
	if !esObjeto || observer == nil {
		t.Fatalf("getViewTreeObserver devolvió %T y se esperaba un objeto no nulo", obs)
	}
	if observer.clase != "Landroid/view/ViewTreeObserver;" {
		t.Errorf("el observador es de %s y se esperaba ViewTreeObserver", observer.clase)
	}
	if ligado, _ := vista.campos["vista.viewTreeObserver"].(*objetoDalvik); ligado != observer {
		t.Error("el observador no quedó ligado a la vista que lo pidió")
	}
	otro, ok, err := m.intentaStub([]string{raiz}, "getViewTreeObserver", proto, nil, vista)
	if err != nil || !ok || otro != observer {
		t.Errorf("la segunda llamada devolvió %v, ok=%v, err=%v; se esperaba el mismo observador",
			otro, ok, err)
	}
	if _, hayAncho := observer.campos[campoLayoutParamsAncho]; hayAncho {
		t.Error("el observador inventó una medida de ancho")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"no hay ciclo de layout ni medidas reales (aproximación)") {
		t.Errorf("getViewTreeObserver no avisó de la aproximación: %s", avisos)
	}
}

// TestViewTreeObserverListenerInerteConAviso comprueba que registrar o quitar un
// `OnGlobalLayoutListener` no ejecuta ningún callback ni inventa medidas: solo deja el aviso de
// aproximación, que es lo que impide confundir el observador inerte con un layout real.
func TestViewTreeObserverListenerInerteConAviso(t *testing.T) {
	m := maquinaDeStubs()
	observer := objDe("Landroid/view/ViewTreeObserver;")
	listener := objDe("Lcom/termux/app/terminal/TermuxActivityRootView;")
	proto := "(Landroid/view/ViewTreeObserver$OnGlobalLayoutListener;)V"
	for _, nombre := range []string{"addOnGlobalLayoutListener", "removeOnGlobalLayoutListener"} {
		v, ok, err := m.intentaStub([]string{"Landroid/view/ViewTreeObserver;"}, nombre, proto,
			[]any{listener}, observer)
		if err != nil || !ok || v != nil {
			t.Errorf("%s: v=%v ok=%v err=%v", nombre, v, ok, err)
		}
	}
	if len(listener.campos) != 0 {
		t.Error("el listener recibió estado: el modelo no debe ejecutar callbacks de layout")
	}
	avisos := strings.Join(m.avisos, "\n")
	if !strings.Contains(avisos, "listener inerte, sin callbacks de layout (aproximación)") {
		t.Errorf("no se avisó de que el listener es inerte: %s", avisos)
	}
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna llamada cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestViewTreeObserverReceptorNuloSigueCortando mantiene el guardián de `invoca`: aunque el stub
// del listener exista, un receptor nulo —el cero que causaba la frontera— sigue cortando con el
// error de siempre, antes de llegar al stub.
func TestViewTreeObserverReceptorNuloSigueCortando(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Landroid/view/ViewTreeObserver;", name: "addOnGlobalLayoutListener",
		proto: dexProto{params: []string{"Landroid/view/ViewTreeObserver$OnGlobalLayoutListener;"}},
	}}}
	listener := objDe("Lcom/termux/app/terminal/TermuxActivityRootView;")
	_, err := m.invoca(0x6e, d, 0, []int{0, 1}, []any{nil, listener},
		"Lcom/termux/app/TermuxActivity;")
	if err == nil {
		t.Fatal("addOnGlobalLayoutListener con receptor nulo no devolvió error")
	}
	esperado := "se llamó a android.view.ViewTreeObserver.addOnGlobalLayoutListener sobre un objeto nulo"
	if err.Error() != esperado {
		t.Errorf("el error fue %q y se esperaba %q", err, esperado)
	}
}

// TestAudioAttributesBuilderNoNulo falsifica la frontera de los 7126 pasos de Termux. El dex de
// `getBellSoundPool` crea el builder con `new-instance` y llama a su `<init>`, que vive en el
// framework y no tiene cuerpo en el dex. El constructor tiene que entregar la referencia modelada
// del builder, marcada como inerte, sin inicializar ningún atributo ni crear sonido.
func TestAudioAttributesBuilderNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/AudioAttributes$Builder;")
	v, ok, err := m.intentaStub([]string{"Landroid/media/AudioAttributes$Builder;"},
		"<init>", "()V", nil, builder)
	if err != nil || !ok {
		t.Fatalf("AudioAttributes.Builder.<init>: v=%v ok=%v err=%v", v, ok, err)
	}
	o, esObjeto := v.(*objetoDalvik)
	if !esObjeto || o == nil {
		t.Fatalf("<init> devolvió %T y se esperaba un objeto no nulo", v)
	}
	if o != builder {
		t.Error("el constructor no devolvió la referencia modelada del builder")
	}
	if o.clase != "Landroid/media/AudioAttributes$Builder;" {
		t.Errorf("el builder es de %s y se esperaba AudioAttributes$Builder", o.clase)
	}
	if o.campos[markerInerteAudio] == nil {
		t.Error("el builder no quedó marcado como inerte")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"constructor inerte, sin audio real (aproximación)") {
		t.Errorf("el constructor no avisó de la aproximación: %s", avisos)
	}
}

// TestAudioAttributesSetterInerteConAviso comprueba que los cuatro setters del builder devuelven
// el MISMO builder —no el cero que dejaba el registro nulo y cortaba en `setUsage`— y dejan un
// aviso explícito de aproximación. Ninguno toca el sonido ni guarda atributos reales.
func TestAudioAttributesSetterInerteConAviso(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/AudioAttributes$Builder;")
	builder.campos[markerInerteAudio] = builder.clase
	protos := map[string]string{
		"setUsage":            "(I)Landroid/media/AudioAttributes$Builder;",
		"setContentType":      "(I)Landroid/media/AudioAttributes$Builder;",
		"setFlags":            "(II)Landroid/media/AudioAttributes$Builder;",
		"setLegacyStreamType": "(I)Landroid/media/AudioAttributes$Builder;",
	}
	for nombre, proto := range protos {
		v, ok, err := m.intentaStub([]string{"Landroid/media/AudioAttributes$Builder;"},
			nombre, proto, []any{int32(4), int32(0)}, builder)
		if err != nil || !ok {
			t.Fatalf("%s: v=%v ok=%v err=%v", nombre, v, ok, err)
		}
		if v != builder {
			t.Errorf("%s devolvió %v y se esperaba el mismo builder para encadenar", nombre, v)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"setter inerte, sin audio real (aproximación)") {
		t.Errorf("los setters no avisaron de la aproximación: %s", avisos)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos,
		"no está en el dex ni en los stubs") {
		t.Errorf("algún setter cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestAudioAttributesReceptorNuloSigueCortando congela el guardián de `invoca`: el stub de los
// setters no lo relaja, así que el cero que causaba la frontera sigue cortando con el error de
// siempre antes de llegar al stub.
func TestAudioAttributesReceptorNuloSigueCortando(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Landroid/media/AudioAttributes$Builder;", name: "setUsage",
		proto: dexProto{params: []string{"I"}, returnType: "Landroid/media/AudioAttributes$Builder;"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0, 1}, []any{nil, int32(13)},
		"Lcom/termux/app/TermuxActivity;")
	if err == nil {
		t.Fatal("setUsage con receptor nulo no devolvió error")
	}
	esperado := "se llamó a android.media.AudioAttributes$Builder.setUsage sobre un objeto nulo"
	if err.Error() != esperado {
		t.Errorf("el error fue %q y se esperaba %q", err, esperado)
	}
}

// TestAudioAttributesBuildNoNulo comprueba que `build()` entrega un AudioAttributes modelado no
// nulo, marcado como inerte, con su aviso de aproximación y sin sonido detrás.
func TestAudioAttributesBuildNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/AudioAttributes$Builder;")
	builder.campos[markerInerteAudio] = builder.clase
	v, ok, err := m.intentaStub([]string{"Landroid/media/AudioAttributes$Builder;"},
		"build", "()Landroid/media/AudioAttributes;", nil, builder)
	if err != nil || !ok {
		t.Fatalf("build: v=%v ok=%v err=%v", v, ok, err)
	}
	attrs, esObjeto := v.(*objetoDalvik)
	if !esObjeto || attrs == nil {
		t.Fatalf("build devolvió %T y se esperaba un objeto no nulo", v)
	}
	if attrs.clase != "Landroid/media/AudioAttributes;" {
		t.Errorf("el resultado es de %s y se esperaba AudioAttributes", attrs.clase)
	}
	if attrs.campos[markerInerteAudio] == nil {
		t.Error("el AudioAttributes devuelto no quedó marcado como inerte")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"objeto inerte, sin audio real (aproximación)") {
		t.Errorf("build no avisó de la aproximación: %s", avisos)
	}
}

// TestSoundPoolBuilderNoNulo falsifica la frontera de los 7130 pasos de Termux. El dex de
// `getBellSoundPool` crea el builder con `new-instance` y llama a su `<init>`, que vive en el
// framework (dexdump confirma que no hay class_def ni cuerpo para SoundPool$Builder en el dex).
// El constructor tiene que entregar la referencia modelada del builder, marcada como inerte, sin
// inicializar ningún parámetro de sonido ni crear sonido.
func TestSoundPoolBuilderNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/SoundPool$Builder;")
	v, ok, err := m.intentaStub([]string{"Landroid/media/SoundPool$Builder;"},
		"<init>", "()V", nil, builder)
	if err != nil || !ok {
		t.Fatalf("SoundPool.Builder.<init>: v=%v ok=%v err=%v", v, ok, err)
	}
	o, esObjeto := v.(*objetoDalvik)
	if !esObjeto || o == nil {
		t.Fatalf("<init> devolvió %T y se esperaba un objeto no nulo", v)
	}
	if o != builder {
		t.Error("el constructor no devolvió la referencia modelada del builder")
	}
	if o.clase != "Landroid/media/SoundPool$Builder;" {
		t.Errorf("el builder es de %s y se esperaba SoundPool$Builder", o.clase)
	}
	if o.campos[markerInerteAudio] == nil {
		t.Error("el builder no quedó marcado como inerte")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"constructor inerte, sin audio real (aproximación)") {
		t.Errorf("el constructor no avisó de la aproximación: %s", avisos)
	}
}

// TestSoundPoolSetterInerteConAviso comprueba que `setAudioAttributes` y `setMaxStreams` devuelven
// el MISMO builder —no el cero que dejaba el registro nulo y cortaba en `setAudioAttributes`— y
// dejan un aviso explícito de aproximación. Ninguno toca el sonido ni cuenta streams reales.
func TestSoundPoolSetterInerteConAviso(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/SoundPool$Builder;")
	builder.campos[markerInerteAudio] = builder.clase
	protos := map[string]string{
		"setAudioAttributes": "(Landroid/media/AudioAttributes;)Landroid/media/SoundPool$Builder;",
		"setMaxStreams":      "(I)Landroid/media/SoundPool$Builder;",
	}
	for nombre, proto := range protos {
		v, ok, err := m.intentaStub([]string{"Landroid/media/SoundPool$Builder;"},
			nombre, proto, []any{objetoInerte("Landroid/media/AudioAttributes;", markerInerteAudio), int32(1)}, builder)
		if err != nil || !ok {
			t.Fatalf("%s: v=%v ok=%v err=%v", nombre, v, ok, err)
		}
		if v != builder {
			t.Errorf("%s devolvió %v y se esperaba el mismo builder para encadenar", nombre, v)
		}
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"setter inerte, sin audio real (aproximación)") {
		t.Errorf("los setters no avisaron de la aproximación: %s", avisos)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos,
		"no está en el dex ni en los stubs") {
		t.Errorf("algún setter cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestSoundPoolReceptorNuloSigueCortando congela el guardián de `invoca`: el stub de los setters
// no lo relaja, así que el cero que causaba la frontera sigue cortando con el error de siempre
// antes de llegar al stub.
func TestSoundPoolReceptorNuloSigueCortando(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Landroid/media/SoundPool$Builder;", name: "setAudioAttributes",
		proto: dexProto{params: []string{"Landroid/media/AudioAttributes;"},
			returnType: "Landroid/media/SoundPool$Builder;"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0, 1},
		[]any{nil, objetoInerte("Landroid/media/AudioAttributes;", markerInerteAudio)},
		"Lcom/termux/app/TermuxActivity;")
	if err == nil {
		t.Fatal("setAudioAttributes con receptor nulo no devolvió error")
	}
	esperado := "se llamó a android.media.SoundPool$Builder.setAudioAttributes sobre un objeto nulo"
	if err.Error() != esperado {
		t.Errorf("el error fue %q y se esperaba %q", err, esperado)
	}
}

// TestSoundPoolBuildNoNulo comprueba que `build()` entrega un SoundPool modelado no nulo, marcado
// como inerte, con su aviso de aproximación y sin sonido detrás.
func TestSoundPoolBuildNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	builder := objDe("Landroid/media/SoundPool$Builder;")
	builder.campos[markerInerteAudio] = builder.clase
	v, ok, err := m.intentaStub([]string{"Landroid/media/SoundPool$Builder;"},
		"build", "()Landroid/media/SoundPool;", nil, builder)
	if err != nil || !ok {
		t.Fatalf("build: v=%v ok=%v err=%v", v, ok, err)
	}
	sonido, esObjeto := v.(*objetoDalvik)
	if !esObjeto || sonido == nil {
		t.Fatalf("build devolvió %T y se esperaba un objeto no nulo", v)
	}
	if sonido.clase != "Landroid/media/SoundPool;" {
		t.Errorf("el resultado es de %s y se esperaba SoundPool", sonido.clase)
	}
	if sonido.campos[markerInerteAudio] == nil {
		t.Error("el SoundPool devuelto no quedó marcado como inerte")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"objeto inerte, sin audio real (aproximación)") {
		t.Errorf("build no avisó de la aproximación: %s", avisos)
	}
}

// TestAccesibilidadServicioNoNulo falsifica la frontera del constructor de TerminalView
// (dexdump de classes14.dex, 013cf0-013d08): Termux pide `getSystemService("accessibility")`,
// hace `check-cast` a AccessibilityManager y acto seguido llama a `isEnabled`. En Android el
// servicio existe aunque no haya servicios de accesibilidad; el cero devolvía un receptor nulo
// y cortaba la construcción de la vista. Solo se atiende ese nombre y esa firma exacta.
func TestAccesibilidadServicioNoNulo(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	proto := "(Ljava/lang/String;)Ljava/lang/Object;"
	v, ok, err := m.intentaStub([]string{"Lcom/termux/app/TermuxActivity;"}, "getSystemService",
		proto, []any{"accessibility"}, actividad)
	if err != nil || !ok {
		t.Fatalf("getSystemService(accessibility): v=%v ok=%v err=%v", v, ok, err)
	}
	manager, esObjeto := v.(*objetoDalvik)
	if !esObjeto || manager == nil {
		t.Fatalf("getSystemService devolvió %T y se esperaba un objeto no nulo", v)
	}
	if manager.clase != "Landroid/view/accessibility/AccessibilityManager;" {
		t.Errorf("el servicio es de %s y se esperaba AccessibilityManager", manager.clase)
	}
	if manager.campos[markerInerteAccesibilidad] == nil {
		t.Error("el gestor no quedó marcado como inerte")
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"sin servicios de accesibilidad reales (aproximación)") {
		t.Errorf("getSystemService no avisó de la aproximación: %s", avisos)
	}
}

// TestAccesibilidadIsEnabledCero comprueba que `isEnabled:()Z` sobre el gestor modelado
// devuelve `int32(0)`, que es el cero de `Z` que TerminalView guarda en `mAccessibilityEnabled`.
// No se inventa ningún servicio activo.
func TestAccesibilidadIsEnabledCero(t *testing.T) {
	m := maquinaDeStubs()
	manager := objetoInerte("Landroid/view/accessibility/AccessibilityManager;",
		markerInerteAccesibilidad)
	v, ok, err := m.intentaStub([]string{"Landroid/view/accessibility/AccessibilityManager;"},
		"isEnabled", "()Z", nil, manager)
	if err != nil || !ok {
		t.Fatalf("isEnabled: v=%v ok=%v err=%v", v, ok, err)
	}
	if v != int32(0) {
		t.Errorf("isEnabled devolvió %T(%v) y se esperaba int32(0)", v, v)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"sin servicios de accesibilidad reales (aproximación)") {
		t.Errorf("isEnabled no avisó de la aproximación: %s", avisos)
	}
}

// TestAccesibilidadOtroServicioCaeAlCero congela el alcance: `getSystemService` solo responde
// por "accessibility"; cualquier otro nombre sigue sin stub y `invoca` entrega el cero de
// `Ljava/lang/Object;` con el aviso de siempre.
func TestAccesibilidadOtroServicioCaeAlCero(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	_, ok, err := m.intentaStub([]string{"Lcom/termux/app/TermuxActivity;"}, "getSystemService",
		"(Ljava/lang/String;)Ljava/lang/Object;", []any{"wifi"}, actividad)
	if err != nil || ok {
		t.Fatalf("getSystemService(wifi) no debía atenderlo: ok=%v err=%v", ok, err)
	}
	d := &dexFile{methods: []dexMethod{{
		owner: "Landroid/content/Context;", name: "getSystemService",
		proto: dexProto{params: []string{"Ljava/lang/String;"},
			returnType: "Ljava/lang/Object;"},
	}}}
	v, err := m.invoca(0x6e, d, 0, []int{0, 1}, []any{actividad, "wifi"},
		"Lcom/termux/app/TermuxActivity;")
	if err != nil {
		t.Fatalf("getSystemService(wifi) devolvió error: %v", err)
	}
	if v != nil {
		t.Errorf("getSystemService(wifi) devolvió %v y se esperaba el cero (nil)", v)
	}
	if avisos := strings.Join(m.avisos, "\n"); !strings.Contains(avisos,
		"no está en el dex ni en los stubs") {
		t.Errorf("otro servicio no cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestAccesibilidadReceptorNuloSigueCortando mantiene el guardián de `invoca`: el stub no
// relaja el error de siempre cuando el receptor es el cero que causaba la frontera.
func TestAccesibilidadReceptorNuloSigueCortando(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Landroid/view/accessibility/AccessibilityManager;", name: "isEnabled",
		proto: dexProto{returnType: "Z"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "Lcom/termux/app/TermuxActivity;")
	if err == nil {
		t.Fatal("isEnabled con receptor nulo no devolvió error")
	}
	esperado := "se llamó a android.view.accessibility.AccessibilityManager.isEnabled " +
		"sobre un objeto nulo"
	if err.Error() != esperado {
		t.Errorf("el error fue %q y se esperaba %q", err, esperado)
	}
}
