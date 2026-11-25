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
	"encoding/json"
	"errors"
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
