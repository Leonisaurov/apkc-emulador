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
