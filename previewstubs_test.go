package main

// Pruebas de los stubs que las apps de ejemplo destaparon: ArrayList, BluetoothAdapter, el reloj
// monotónico y la cancelación de tareas de un Handler.
//
// Las cuatro APIs se emulan con estado por instancia y reloj lógico, sin red ni dispositivo: el
// ArrayList guarda sus elementos, el adaptador de Bluetooth se modela encendido y con cero equipos
// vinculados, `SystemClock.elapsedRealtime` lee el reloj de la vista previa y `removeCallbacks`
// retira de la cola las tareas del mismo Runnable. Cada prueba comprueba que la llamada NO cae en
// el aviso de «no está en el dex ni en los stubs» y que dos ejecuciones del mismo guion dan el
// mismo JSON.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// sinAvisoDeStub falla si la ejecución avisó de una API que el intérprete no reconoce: es el síntoma
// que tenían estas cuatro antes de tener su stub.
func sinAvisoDeStub(t *testing.T, m *modeloVista) {
	t.Helper()
	for _, a := range m.Ejecucion.Avisos {
		if strings.Contains(a, "no está en el dex ni en los stubs") {
			t.Errorf("la API cayó en el aviso de stub ausente: %s", a)
		}
	}
}

// TestHilosSinteticosDelPreview comprueba la cadena que paraba a Libre Memory Game: el guardián de
// receptor nulo corta antes de cualquier stub, así que `currentThread()` tiene que devolver un
// Thread no nulo sobre el que `getStackTrace()` pueda responderse. `Looper.getThread()` lleva al
// mismo hilo sintético y `uptimeMillis()` lee el reloj lógico en vez del cero.
func TestHilosSinteticosDelPreview(t *testing.T) {
	m := maquinaDeStubs()
	hilo, ok, err := m.intentaStub([]string{"Ljava/lang/Thread;"},
		"currentThread", "()Ljava/lang/Thread;", nil, nil)
	if err != nil || !ok {
		t.Fatalf("currentThread: ok=%v err=%v", ok, err)
	}
	esperaObjetoDeStub(t, hilo, "Ljava/lang/Thread;")

	traza, ok, err := m.intentaStub([]string{"Ljava/lang/Thread;"},
		"getStackTrace", "()[Ljava/lang/StackTraceElement;", nil, hilo)
	if err != nil || !ok {
		t.Fatalf("getStackTrace: ok=%v err=%v", ok, err)
	}
	arreglo, esArreglo := traza.(*arregloDalvik)
	if !esArreglo || arreglo == nil {
		t.Fatalf("getStackTrace devolvió %T y se esperaba un arreglo no nulo", traza)
	}
	if arreglo.tipo != "[Ljava/lang/StackTraceElement;" {
		t.Errorf("el arreglo es de %s y se esperaba [Ljava/lang/StackTraceElement;", arreglo.tipo)
	}
	// Kotlin lee la primera casilla para formar el mensaje de un parámetro nulo, así que el arreglo
	// no puede estar vacío: con cero casillas la app caía en «aget con el índice 0 fuera del arreglo».
	if len(arreglo.datos) != 1 {
		t.Fatalf("la traza sintética tiene %d elementos y se esperaba 1", len(arreglo.datos))
	}
	if elemento, ok := arreglo.datos[0].(*objetoDalvik); !ok || elemento == nil ||
		elemento.clase != "Ljava/lang/StackTraceElement;" {
		t.Errorf("la primera casilla es %T y se esperaba un StackTraceElement", arreglo.datos[0])
	}

	looper := objDe("Landroid/os/Looper;")
	delLooper, ok, err := m.intentaStub([]string{"Landroid/os/Looper;"},
		"getThread", "()Ljava/lang/Thread;", nil, looper)
	if err != nil || !ok {
		t.Fatalf("getThread: ok=%v err=%v", ok, err)
	}
	esperaObjetoDeStub(t, delLooper, "Ljava/lang/Thread;")
	if delLooper != hilo {
		t.Error("getThread devolvió un hilo distinto de currentThread: AndroidX lo tomaría por otro hilo")
	}
	otroHilo, ok, err := m.intentaStub([]string{"Ljava/lang/Thread;"},
		"currentThread", "()Ljava/lang/Thread;", nil, nil)
	if err != nil || !ok || otroHilo != hilo {
		t.Error("currentThread no devolvió siempre la misma referencia")
	}

	reloj := int64(1234)
	m.reloj = &reloj
	u, ok, err := m.intentaStub([]string{"Landroid/os/SystemClock;"},
		"uptimeMillis", "()J", nil, nil)
	if err != nil || !ok {
		t.Fatalf("uptimeMillis: ok=%v err=%v", ok, err)
	}
	if u != reloj {
		t.Errorf("uptimeMillis devolvió %v y se esperaba el reloj %d", u, reloj)
	}
	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("algún hilo cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestHilosElGuardianSigueCortandoAntesDelStub: los stubs nuevos no pueden servir para saltarse el
// guardián de receptor nulo. Con `currentThread()` apagado, el cero llega a `getStackTrace()` y el
// error sigue siendo el de siempre, no una traza inventada.
func TestHilosElGuardianSigueCortandoAntesDelStub(t *testing.T) {
	stubsFrameworkActivos = false
	defer func() { stubsFrameworkActivos = true }()

	m := maquinaDeStubs()
	m.prog = &programaDalvik{clases: map[string]*claseDalvik{}}
	d := &dexFile{methods: []dexMethod{{
		owner: "Ljava/lang/Thread;", name: "getStackTrace",
		proto: dexProto{returnType: "[Ljava/lang/StackTraceElement;"},
	}}}
	_, err := m.invoca(0x6e, d, 0, []int{0}, []any{nil}, "")
	if err == nil {
		t.Fatal("getStackTrace con receptor nulo no devolvió error")
	}
	if !strings.Contains(err.Error(), "se llamó a java.lang.Thread.getStackTrace sobre un objeto nulo") {
		t.Errorf("el error fue %q y se esperaba el del guardián de receptor nulo", err)
	}
}

// mismaEjecucion reejecuta el guion y exige el mismo JSON: el reloj lógico y el estado de los stubs
// no pueden depender de nada de fuera del guion.
func mismaEjecucion(t *testing.T, apk []byte, eventos []eventoUsuario) {
	t.Helper()
	primera, err := modeloVistaDeAPKConEventos(apk, "", true, eventos)
	if err != nil {
		t.Fatalf("la primera ejecución falló: %v", err)
	}
	segunda, err := modeloVistaDeAPKConEventos(apk, "", true, eventos)
	if err != nil {
		t.Fatalf("la segunda ejecución falló: %v", err)
	}
	jsonPrimera, err := json.Marshal(primera)
	if err != nil {
		t.Fatalf("el JSON de la primera ejecución: %v", err)
	}
	jsonSegunda, err := json.Marshal(segunda)
	if err != nil {
		t.Fatalf("el JSON de la segunda ejecución: %v", err)
	}
	if string(jsonPrimera) != string(jsonSegunda) {
		t.Errorf("dos ejecuciones del mismo guion dan JSON distinto:\n%s\n%s", jsonPrimera, jsonSegunda)
	}
}

// fuenteLista usa el ArrayList como lo harían `notas`/`tareas`: añade, inserta en un índice, lee
// por índice, cuenta, vacía y pregunta si está vacío, con dos instancias para que el estado no se
// comparta.
const fuenteLista = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    ArrayList lista;
    ArrayList otra;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        preparar();
        String resumen = describir() + vaciar();
        total.setText(resumen);
    }

    private void preparar() {
        lista = new ArrayList();
        lista.add("uno");
        lista.add("dos");
        lista.add("tres");
        lista.add(1, "medio");
        otra = new ArrayList();
        otra.add("x");
    }

    private String describir() {
        String texto = primeros();
        texto = texto + "," + tercero();
        return texto + ",tam" + tamanos();
    }

    private String primeros() {
        return leer(0) + "," + leer(1);
    }

    private String tercero() {
        return leer(3);
    }

    private String leer(int i) {
        return (String) lista.get(i);
    }

    private String tamanos() {
        return "" + lista.size() + "/" + otra.size();
    }

    private String vaciar() {
        lista.clear();
        String texto = "";
        if (lista.isEmpty() == true) {
            texto = ",vacia";
        }
        return texto;
    }
}
`

// TestElArrayListGuardaSuEstadoPorInstancia: el índice manda (el `add(1, …)` queda en medio), el
// tamaño es el de cada lista, `clear` vacía solo la suya y `isEmpty` lo dice.
func TestPreviewElArrayListGuardaSuEstadoPorInstancia(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteLista})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "uno,medio,tres,tam4/1,vacia" {
		t.Errorf("el ArrayList dejó %q y se esperaba «uno,medio,tres,tam4/1,vacia»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}

// fuenteBluetooth recorre el camino de `chatbt`: adaptador presente y encendido, cero equipos
// vinculados, un iterador vacío y el descubrimiento lanzado.
const fuenteBluetooth = `package com.hola.reloj;

import android.app.*;
import android.bluetooth.BluetoothAdapter;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    TextView total;
    BluetoothAdapter adaptador;
    String texto;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        texto = "sin adaptador";
        adaptador = BluetoothAdapter.getDefaultAdapter();
        if (adaptador != null) {
            comprobar();
        }
        total.setText(texto);
    }

    private void comprobar() {
        texto = "apagado";
        if (adaptador.isEnabled() == true) {
            listar();
        }
    }

    private void listar() {
        texto = "listo";
        int cuantos = contar();
        if (hayEquipos() == true) {
            texto = texto + " con equipos";
        }
        texto = texto + " · " + cuantos + " vínculos";
        if (adaptador.startDiscovery() == true) {
            texto = texto + " · buscando";
        }
    }

    private int contar() {
        Set enlazados = adaptador.getBondedDevices();
        return enlazados.size();
    }

    private boolean hayEquipos() {
        Set enlazados = adaptador.getBondedDevices();
        Iterator it = enlazados.iterator();
        return it.hasNext();
    }
}
`

// TestElAdaptadorDeBluetoothModelaSuEstado: `getDefaultAdapter` no es nulo, `isEnabled` dice que sí,
// `getBondedDevices` rinde un conjunto vacío y `startDiscovery` devuelve true sin abrir nada.
func TestPreviewElAdaptadorDeBluetoothModelaSuEstado(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteBluetooth})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "listo · 0 vínculos · buscando" {
		t.Errorf("el adaptador de Bluetooth dejó %q y se esperaba «listo · 0 vínculos · buscando»; "+
			"avisos %v", got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}

// fuenteCancela postea el mismo Runnable dos veces y lo retira con `removeCallbacks`: el primer
// plazo corre y el segundo no.
const fuenteCancela = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity implements Runnable {
    TextView total;
    Handler manejador;
    int pulsos = 0;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        manejador = new Handler();
        int arranque = (int) SystemClock.elapsedRealtime();
        total.setText("t=" + arranque);
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                parar();
            }
        });
        programar();
    }

    private void programar() {
        manejador.postDelayed(this, 100L);
        manejador.postDelayed(this, 200L);
    }

    private void parar() {
        manejador.removeCallbacks(this);
    }

    public void run() {
        pulsos = pulsos + 1;
        int ahora = (int) SystemClock.elapsedRealtime();
        total.setText("pulso " + pulsos + " t=" + ahora);
    }
}
`

// TestElRelojLogicoYElRemoveCallbacks: a los 100 ms corre el primer plazo con el reloj lógico en
// 100, el clic retira el de 200 ms y el avance siguiente no encuentra nada. Dos ejecuciones dan el
// mismo JSON.
func TestPreviewElRelojLogicoYElRemoveCallbacks(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteCancela})
	guion := []eventoUsuario{
		{Tipo: "avanzar", Dato: "100"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "500"},
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("el guion falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "pulso 1 t=100" {
		t.Errorf("el cronómetro dejó %q y se esperaba «pulso 1 t=100»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	if m.Ejecucion.Tareas != 1 {
		t.Errorf("la cola ejecutó %d tareas y se esperaba 1: el removeCallbacks no retiró la del "+
			"segundo plazo", m.Ejecucion.Tareas)
	}
	if m.Ejecucion.Reloj != 600 {
		t.Errorf("el reloj quedó en %d y se esperaba 600", m.Ejecucion.Reloj)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, guion)
}

// fuenteCancelaTodo postea dos Runnables distintos y los retira todos con
// `removeCallbacksAndMessages(null)`, que es el botón de reiniciar.
const fuenteCancelaTodo = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.view.*;
import android.widget.*;

public class Casos extends Activity {
    TextView total;
    Handler manejador;

    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        total = (TextView) findViewById(R.id.total);
        manejador = new Handler();
        total.setText("en cero");
        findViewById(R.id.pulsar).setOnClickListener(new View.OnClickListener() {
            public void onClick(View v) {
                manejador.removeCallbacksAndMessages(null);
            }
        });
        manejador.postDelayed(new Uno(total), 100L);
        manejador.postDelayed(new Dos(total), 200L);
    }
}
`

// fuenteUno y fuenteDos son los dos Runnables que el botón debe cancelar.
const fuenteUno = `package com.hola.reloj;

import android.widget.*;

public class Uno implements Runnable {
    TextView total;

    public Uno(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("uno");
    }
}
`

const fuenteDos = `package com.hola.reloj;

import android.widget.*;

public class Dos implements Runnable {
    TextView total;

    public Dos(TextView t) {
        total = t;
    }

    public void run() {
        total.setText("dos");
    }
}
`

// TestRemoveCallbacksAndMessagesVaciaLaCola: cancelado antes de vencer, ninguno de los dos plazos
// deja su texto y la cola ejecuta cero tareas.
func TestPreviewRemoveCallbacksAndMessagesVaciaLaCola(t *testing.T) {
	apk := apkConFuentes(t,
		fuenteJava{nombre: "Casos.java", contenido: fuenteCancelaTodo},
		fuenteJava{nombre: "Uno.java", contenido: fuenteUno},
		fuenteJava{nombre: "Dos.java", contenido: fuenteDos},
	)
	guion := []eventoUsuario{
		{Tipo: "avanzar", Dato: "50"},
		{Tipo: "pulsar", Dato: "id/pulsar"},
		{Tipo: "avanzar", Dato: "500"},
	}
	m, err := modeloVistaDeAPKConEventos(apk, "", true, guion)
	if err != nil {
		t.Fatalf("el guion falló: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "en cero" {
		t.Errorf("la cola cancelada dejó %q y se esperaba «en cero»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	if m.Ejecucion.Tareas != 0 {
		t.Errorf("la cola ejecutó %d tareas y se esperaba ninguna", m.Ejecucion.Tareas)
	}
	if m.Ejecucion.Reloj != 550 {
		t.Errorf("el reloj quedó en %d y se esperaba 550", m.Ejecucion.Reloj)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, guion)
}

// fuenteBluetoothSinRadio llama a las dos APIs que android-34 deja invocar sobre el adaptador:
// `listenUsingRfcommWithServiceRecord` y `getRemoteDevice`. `createRfcommSocketToServiceRecord` se
// mudó a `BluetoothDevice` en el SDK, así que esa la cubre la llamada directa al stub.
const fuenteBluetoothSinRadio = `package com.hola.reloj;

import android.app.*;
import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.bluetooth.BluetoothServerSocket;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        BluetoothAdapter adaptador = BluetoothAdapter.getDefaultAdapter();
        UUID uuid = null;
        BluetoothServerSocket servidor = adaptador.listenUsingRfcommWithServiceRecord("eco", uuid);
        BluetoothDevice equipo = adaptador.getRemoteDevice("00:11:22:33:44:55");
        String texto = "";
        if (servidor == null) {
            texto = texto + "servidor nulo";
        }
        if (equipo == null) {
            texto = texto + ",equipo nulo";
        }
        total.setText(texto);
    }
}
`

// TestPreviewRechazaBluetoothSinRadio: las tres APIs sin radio se rechazan por su nombre con el
// motivo de la frontera y devuelven el cero de SU tipo de retorno (socket, servidor de sockets y
// dispositivo), no el de un BluetoothSocket para todas. `createRfcommSocketToServiceRecord` se
// cubre con una llamada directa porque android-34 la expone en `BluetoothDevice`, no en el
// adaptador; las otras dos se ejecutan además dentro de una app.
func TestPreviewRechazaBluetoothSinRadio(t *testing.T) {
	m := &maquinaDalvik{vistos: map[string]bool{}}
	adaptador := &objetoDalvik{clase: "Landroid/bluetooth/BluetoothAdapter;",
		campos: map[string]any{}}
	casos := []struct{ nombre, proto, cero string }{
		{"createRfcommSocketToServiceRecord",
			"(Ljava/util/UUID;)Landroid/bluetooth/BluetoothSocket;", "Landroid/bluetooth/BluetoothSocket;"},
		{"listenUsingRfcommWithServiceRecord",
			"(Ljava/lang/String;Ljava/util/UUID;)Landroid/bluetooth/BluetoothServerSocket;",
			"Landroid/bluetooth/BluetoothServerSocket;"},
		{"getRemoteDevice",
			"(Ljava/lang/String;)Landroid/bluetooth/BluetoothDevice;", "Landroid/bluetooth/BluetoothDevice;"},
	}
	for _, c := range casos {
		v, ok, err := m.intentaStub([]string{"Landroid/bluetooth/BluetoothAdapter;"},
			c.nombre, c.proto, nil, adaptador)
		if err != nil {
			t.Fatalf("%s devolvió error: %v", c.nombre, err)
		}
		if !ok {
			t.Fatalf("%s no llegó al stub de Bluetooth", c.nombre)
		}
		if v != nil {
			t.Errorf("%s devolvió %v y se esperaba el cero de %s", c.nombre, v, c.cero)
		}
	}
	avisos := strings.Join(m.avisos, "\n")
	if strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna API cayó en el aviso de stub ausente: %s", avisos)
	}
	for _, c := range casos {
		if !strings.Contains(avisos, c.nombre) {
			t.Errorf("falta el aviso por nombre de %s: %v", c.nombre, m.avisos)
		}
		if !strings.Contains(avisos, "se devuelve el cero de "+c.cero) {
			t.Errorf("falta el cero de %s: %v", c.cero, m.avisos)
		}
	}
	if !strings.Contains(avisos, "no hay radio ni red") {
		t.Errorf("el aviso no dice el motivo de la frontera: %v", m.avisos)
	}

	// y las dos que android-34 deja invocar sobre el adaptador, dentro de una app de verdad
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteBluetoothSinRadio})
	vista, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, vista, idTotal); got != "servidor nulo,equipo nulo" {
		t.Errorf("los rechazos dejaron %q y se esperaba «servidor nulo,equipo nulo»; avisos %v",
			got, vista.Ejecucion.Avisos)
	}
	if !strings.Contains(strings.Join(vista.Ejecucion.Avisos, "\n"), "no hay radio ni red") {
		t.Errorf("la app no recibió el motivo de la frontera: %v", vista.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, vista)
	mismaEjecucion(t, apk, nil)
}

// fuenteListaFueraDeRango pide el índice 9 de una lista de dos dentro de un try: la excepción que
// lanza el stub tiene que llegar al catch del preview como cualquier excepción del dex.
const fuenteListaFueraDeRango = `package com.hola.reloj;

import android.app.*;
import android.os.*;
import android.widget.*;
import java.util.*;

public class Casos extends Activity {
    public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.main);
        TextView total = (TextView) findViewById(R.id.total);
        ArrayList lista = new ArrayList();
        lista.add("uno");
        lista.add("dos");
        String texto = "sin excepcion";
        try {
            Object cual = lista.get(9);
            texto = "no salto";
        } catch (IndexOutOfBoundsException e) {
            texto = "capturada";
        }
        total.setText(texto);
    }
}
`

// TestPreviewCapturaIndexOutOfBoundsDeArrayList: el get(9) de una lista de dos no es un fallo del
// intérprete, es una IndexOutOfBoundsException que el catch del proyecto captura y convierte en
// texto.
func TestPreviewCapturaIndexOutOfBoundsDeArrayList(t *testing.T) {
	apk := apkConFuentes(t, fuenteJava{nombre: "Casos.java", contenido: fuenteListaFueraDeRango})
	m, err := modeloVistaDeAPKConEventos(apk, "", true, nil)
	if err != nil {
		t.Fatalf("el apk no se ejecuta: %v", err)
	}
	if got := textoDeNodo(t, m, idTotal); got != "capturada" {
		t.Errorf("el catch dejó %q y se esperaba «capturada»; avisos %v",
			got, m.Ejecucion.Avisos)
	}
	sinAvisoDeStub(t, m)
	mismaEjecucion(t, apk, nil)
}

// TestTermuxLosAvisosQueSePuedenQuitar recorre los seis avisos del JSON de Termux (§368). Cuatro
// tienen stub propio —createPackageContext, ReportActivity$1.start, Arrays.toString y
// Properties.stringPropertyNames—; los dos de File quedan como dependencia del frente de ROM y la
// prueba exige que NO estén implementados aquí, para no invadir ese territorio.
func TestTermuxLosAvisosQueSePuedenQuitar(t *testing.T) {
	m := maquinaDeStubs()
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad

	ctx, ok, err := m.intentaStub([]string{"Lcom/termux/app/TermuxActivity;"},
		"createPackageContext", "(Ljava/lang/String;I)Landroid/content/Context;",
		[]any{cadenaObjeto("com.termux"), int32(0)}, actividad)
	if err != nil || !ok {
		t.Fatalf("createPackageContext: v=%v ok=%v err=%v", ctx, ok, err)
	}
	esperaObjetoDeStub(t, ctx, "Landroid/content/Context;")
	pm, ok, err := m.intentaStub([]string{"Landroid/content/Context;"},
		"getPackageManager", "()Landroid/content/pm/PackageManager;", nil, ctx)
	if err != nil || !ok || pm == nil {
		t.Fatalf("getPackageManager sobre el contexto devuelto: v=%v ok=%v err=%v", pm, ok, err)
	}

	informe := objDe("Lcom/termux/shared/activities/ReportActivity$1;")
	v, ok, err := m.intentaStub([]string{"Lcom/termux/shared/activities/ReportActivity$1;"},
		"start", "()V", nil, informe)
	if err != nil || !ok || v != nil {
		t.Fatalf("start del informe: v=%v ok=%v err=%v", v, ok, err)
	}

	props := objDe("Ljava/util/Properties;")
	conjunto, ok, err := m.intentaStub([]string{"Ljava/util/Properties;"},
		"stringPropertyNames", "()Ljava/util/Set;", nil, props)
	if err != nil || !ok || conjunto == nil {
		t.Fatalf("stringPropertyNames: v=%v ok=%v err=%v", conjunto, ok, err)
	}
	set := esperaObjetoDeStub(t, conjunto, "Ljava/util/Set;")
	it, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "iterator",
		"()Ljava/util/Iterator;", nil, set)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator sobre el conjunto de propiedades: v=%v ok=%v err=%v", it, ok, err)
	}

	// File.exists y File.isFile son del frente de ROM: aquí se comprueba que siguen SIN stub.
	for _, nombre := range []string{"exists", "isFile"} {
		v, ok, err := m.intentaStub([]string{"Ljava/io/File;"}, nombre, "()Z", nil,
			objDe("Ljava/io/File;"))
		if err != nil || ok || v != nil {
			t.Errorf("%s se resolvió en este frente: v=%v ok=%v err=%v", nombre, v, ok, err)
		}
	}

	if avisos := strings.Join(m.avisos, "\n"); strings.Contains(avisos, "no está en el dex ni en los stubs") {
		t.Errorf("alguna llamada cayó en el aviso de stub ausente: %s", avisos)
	}
}

// TestPreferenciasMemoriaTermuxGuardaYLeeSinFichero recorre la cadena que Termux necesita: pedir el
// SharedPreferences al Context, abrir el editor, escribir tres tipos, aplicar y volver a leer. Todo
// el estado vive en el almacén en memoria de la máquina; la prueba falsifica que no aparece el aviso
// de stub ausente y que el mapa de getAll es consultable sin tocar disco.
func TestPreferenciasMemoriaTermuxGuardaYLeeSinFichero(t *testing.T) {
	m := maquinaDeStubs()
	m.almacen = nuevoAlmacenPreferencias()
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	contexto := []string{"Lcom/termux/app/TermuxActivity;", "Landroid/content/Context;"}

	v, ok, err := m.intentaStub(contexto, "getSharedPreferences",
		"(Ljava/lang/String;I)Landroid/content/SharedPreferences;",
		[]any{cadenaObjeto("termux"), int32(0)}, actividad)
	if err != nil || !ok || v == nil {
		t.Fatalf("getSharedPreferences: v=%v ok=%v err=%v", v, ok, err)
	}
	prefs := esperaObjetoDeStub(t, v, "Landroid/content/SharedPreferences;")
	if prefs.prefsFichero != "termux" {
		t.Fatalf("el almacén se llamó %q y se esperaba termux", prefs.prefsFichero)
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "preferencias-memoria:") {
		t.Errorf("getSharedPreferences no dejó aviso de familia: %v", m.avisos)
	}

	v, ok, err = m.intentaStub([]string{"Landroid/content/SharedPreferences;"}, "edit",
		"()Landroid/content/SharedPreferences$Editor;", nil, prefs)
	if err != nil || !ok || v == nil {
		t.Fatalf("edit: v=%v ok=%v err=%v", v, ok, err)
	}
	editor := esperaObjetoDeStub(t, v, "Landroid/content/SharedPreferences$Editor;")
	escrituras := []struct {
		nombre string
		valor  any
	}{
		{"putString", cadenaObjeto("Hola")},
		{"putInt", int32(14)},
		{"putBoolean", boolAInt(true)},
	}
	for i, e := range escrituras {
		clave := []string{"fuente", "tamano", "activo"}[i]
		v, ok, err := m.intentaStub([]string{"Landroid/content/SharedPreferences$Editor;"}, e.nombre,
			"(Ljava/lang/String;Ljava/lang/Object;)Landroid/content/SharedPreferences$Editor;",
			[]any{cadenaObjeto(clave), e.valor}, editor)
		if err != nil || !ok || v != editor {
			t.Fatalf("%s(%q): v=%v ok=%v err=%v", e.nombre, clave, v, ok, err)
		}
	}
	if len(m.almacen.ficheros["termux"]) != 0 {
		t.Fatalf("el editor escribió antes de apply: %v", m.almacen.ficheros["termux"])
	}
	// Cobertura de esta aserción: solo `os.ReadDir` sobre el directorio de trabajo temporal, no
	// sobre una ruta absoluta. Si `apply` escribiera en el cwd, el directorio dejaría de estar
	// vacío. No se inyecta un escribidor: el intérprete no tiene ningún punto de escritura de
	// preferencias y cablear uno para que nunca se use añadiría código muerto.
	directorio := t.TempDir()
	anteriorDirectorio, err := os.Getwd()
	if err != nil {
		t.Fatalf("el directorio de trabajo: %v", err)
	}
	if err := os.Chdir(directorio); err != nil {
		t.Fatalf("entrar en el directorio de la sonda: %v", err)
	}
	defer func() { _ = os.Chdir(anteriorDirectorio) }()
	if v, ok, err := m.intentaStub([]string{"Landroid/content/SharedPreferences$Editor;"}, "apply",
		"()V", nil, editor); err != nil || !ok || v != nil {
		t.Fatalf("apply: v=%v ok=%v err=%v", v, ok, err)
	}
	entradas, err := os.ReadDir(directorio)
	if err != nil {
		t.Fatalf("leer el directorio de la sonda: %v", err)
	}
	if len(entradas) != 0 {
		t.Fatalf("apply creó %d ficheros en disco: %v", len(entradas), entradas)
	}
	if datos := m.almacen.ficheros["termux"]; len(datos) != 3 || datos["fuente"] != "Hola" {
		t.Errorf("apply no volcó los tres valores al almacén en memoria: %v", datos)
	}

	lector := []string{"Landroid/content/SharedPreferences;"}
	texto, ok, err := m.intentaStub(lector, "getString",
		"(Ljava/lang/String;Ljava/lang/String;)Ljava/lang/String;",
		[]any{cadenaObjeto("fuente"), cadenaObjeto("por defecto")}, prefs)
	if err != nil || !ok {
		t.Fatalf("getString: v=%v ok=%v err=%v", texto, ok, err)
	}
	esperaCadenaDeStub(t, texto, "Hola")
	entero, ok, err := m.intentaStub(lector, "getInt", "(Ljava/lang/String;I)I",
		[]any{cadenaObjeto("tamano"), int32(-1)}, prefs)
	if err != nil || !ok || entero != int32(14) {
		t.Errorf("getInt: v=%v ok=%v err=%v", entero, ok, err)
	}
	booleano, ok, err := m.intentaStub(lector, "getBoolean", "(Ljava/lang/String;Z)Z",
		[]any{cadenaObjeto("activo"), boolAInt(false)}, prefs)
	if err != nil || !ok || booleano != boolAInt(true) {
		t.Errorf("getBoolean: v=%v ok=%v err=%v", booleano, ok, err)
	}

	v, ok, err = m.intentaStub(lector, "getAll", "()Ljava/util/Map;", nil, prefs)
	if err != nil || !ok || v == nil {
		t.Fatalf("getAll: v=%v ok=%v err=%v", v, ok, err)
	}
	mapa := esperaObjetoDeStub(t, v, "Ljava/util/Map;")
	datos, _ := mapa.campos["preferencias.mapa"].(map[string]any)
	if len(datos) != 3 || datos["fuente"] != "Hola" || datos["tamano"] != int32(14) {
		t.Errorf("el mapa de getAll es %v", datos)
	}
	if v, ok, err := m.intentaStub([]string{"Ljava/util/Map;"}, "get",
		"(Ljava/lang/Object;)Ljava/lang/Object;", []any{cadenaObjeto("activo")}, mapa); err != nil || !ok ||
		!boolDe(v) {
		t.Errorf("get sobre el mapa: v=%v ok=%v err=%v", v, ok, err)
	}
	if v, ok, err := m.intentaStub([]string{"Ljava/util/Map;"}, "size", "()I", nil, mapa); err != nil ||
		!ok || v != int32(3) {
		t.Errorf("size del mapa: v=%v ok=%v err=%v", v, ok, err)
	}
	if v, ok, err := m.intentaStub([]string{"Ljava/util/Map;"}, "keySet", "()Ljava/util/Set;", nil, mapa); err != nil ||
		!ok || v == nil {
		t.Fatalf("keySet del mapa: v=%v ok=%v err=%v", v, ok, err)
	} else if claves := esperaObjetoDeStub(t, v, "Ljava/util/Set;").conjunto; len(claves) != 3 {
		t.Errorf("el conjunto de claves es %v", claves)
	} else {
		ordenadas := make([]string, 0, len(claves))
		for _, c := range claves {
			s, _ := cadenaDe(c)
			ordenadas = append(ordenadas, s)
		}
		if strings.Join(ordenadas, ",") != "activo,fuente,tamano" {
			t.Errorf("las claves del mapa son %v y se esperaban ordenadas", ordenadas)
		}
	}
	if !strings.Contains(strings.Join(m.avisos, "\n"), "preferencias-memoria: getAll(") {
		t.Errorf("getAll no dejó aviso de familia: %v", m.avisos)
	}
}

// TestPreferenciasHashMapKeySetSinClavesInventadas falsifica el receptor real de la frontera de
// Termux: un `HashMap` creado en el propio dex, no el mapa de getAll. `keySet` tiene que entregar un
// Set no nulo aunque esté vacío, y después solo las claves que la app puso (en orden fijo). El
// constructor `HashMap(Map)` copia de la misma fuente.
func TestPreferenciasHashMapKeySetSinClavesInventadas(t *testing.T) {
	m := maquinaDeStubs()
	// el `<init>` consulta la jerarquía del dex para decidir si es un Throwable; sin programa el
	// guardión de jerarquía panicaría, así que se le da uno vacío como en las pruebas de stubs.
	m.prog = &programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}
	candidatos := []string{"Ljava/util/HashMap;"}
	mapa := objDe("Ljava/util/HashMap;")

	v, ok, err := m.intentaStub(candidatos, "keySet", "()Ljava/util/Set;", nil, mapa)
	if err != nil || !ok || v == nil {
		t.Fatalf("keySet del HashMap vacío: v=%v ok=%v err=%v", v, ok, err)
	}
	set := esperaObjetoDeStub(t, v, "Ljava/util/Set;")
	if set.conjunto == nil || len(set.conjunto) != 0 {
		t.Fatalf("el HashMap vacío entregó las claves %v", set.conjunto)
	}
	it, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "iterator", "()Ljava/util/Iterator;", nil, set)
	if err != nil || !ok || it == nil {
		t.Fatalf("iterator del Set vacío: v=%v ok=%v err=%v", it, ok, err)
	}
	hasNext, ok, err := m.intentaStub([]string{"Ljava/util/Iterator;"}, "hasNext", "()Z", nil, it)
	if err != nil || !ok || hasNext != int32(0) {
		t.Errorf("hasNext del Set vacío: v=%v ok=%v err=%v", hasNext, ok, err)
	}

	for _, clave := range []string{"z", "a"} {
		if v, ok, err := m.intentaStub(candidatos, "put",
			"(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;",
			[]any{cadenaObjeto(clave), int32(len(clave))}, mapa); err != nil || !ok || v != nil {
			t.Fatalf("put(%q): v=%v ok=%v err=%v", clave, v, ok, err)
		}
	}
	v, ok, err = m.intentaStub(candidatos, "keySet", "()Ljava/util/Set;", nil, mapa)
	if err != nil || !ok || v == nil {
		t.Fatalf("keySet con dos claves: v=%v ok=%v err=%v", v, ok, err)
	}
	claves := esperaObjetoDeStub(t, v, "Ljava/util/Set;").conjunto
	ordenadas := make([]string, 0, len(claves))
	for _, c := range claves {
		s, _ := cadenaDe(c)
		ordenadas = append(ordenadas, s)
	}
	if strings.Join(ordenadas, ",") != "a,z" {
		t.Errorf("las claves son %v y se esperaban a,z", ordenadas)
	}
	if v, ok, err := m.intentaStub(candidatos, "get", "(Ljava/lang/Object;)Ljava/lang/Object;",
		[]any{cadenaObjeto("a")}, mapa); err != nil || !ok || v != int32(1) {
		t.Errorf("get(a): v=%v ok=%v err=%v", v, ok, err)
	}
	if v, ok, err := m.intentaStub(candidatos, "size", "()I", nil, mapa); err != nil || !ok || v != int32(2) {
		t.Errorf("size: v=%v ok=%v err=%v", v, ok, err)
	}

	copia := objDe("Ljava/util/HashMap;")
	if _, ok, err := m.intentaStub(candidatos, "<init>", "(Ljava/util/Map;)V", []any{mapa}, copia); err != nil || !ok {
		t.Fatalf("HashMap(Map): ok=%v err=%v", ok, err)
	}
	v, ok, err = m.intentaStub(candidatos, "keySet", "()Ljava/util/Set;", nil, copia)
	if err != nil || !ok || v == nil {
		t.Fatalf("keySet de la copia: v=%v ok=%v err=%v", v, ok, err)
	}
	if n := len(esperaObjetoDeStub(t, v, "Ljava/util/Set;").conjunto); n != 2 {
		t.Errorf("la copia tiene %d claves y se esperaban 2", n)
	}
}

// TestPreferenciasContainsSobreClavesDelMapa comprueba que `containsKey` y `contains` consultan la
// misma fuente que `keySet`: las claves guardadas en el mapa, nunca un valor inventado. Se prueban
// tanto un HashMap de la app como el Map que devuelve `getAll`.
func TestPreferenciasContainsSobreClavesDelMapa(t *testing.T) {
	m := maquinaDeStubs()
	m.prog = &programaDalvik{dex: &dexFile{}, clases: map[string]*claseDalvik{}}
	m.almacen = nuevoAlmacenPreferencias()
	mapa := objDe("Ljava/util/HashMap;")
	for _, clave := range []string{"z", "a"} {
		if _, ok, err := m.intentaStub([]string{"Ljava/util/HashMap;"}, "put",
			"(Ljava/lang/Object;Ljava/lang/Object;)Ljava/lang/Object;",
			[]any{cadenaObjeto(clave), int32(1)}, mapa); err != nil || !ok {
			t.Fatalf("put(%q): ok=%v err=%v", clave, ok, err)
		}
	}
	for _, caso := range []struct {
		clave    string
		esperado int32
	}{
		{"a", int32(1)}, {"z", int32(1)}, {"ausente", int32(0)},
	} {
		v, ok, err := m.intentaStub([]string{"Ljava/util/Map;"}, "containsKey",
			"(Ljava/lang/Object;)Z", []any{cadenaObjeto(caso.clave)}, mapa)
		if err != nil || !ok || v != caso.esperado {
			t.Errorf("containsKey(%q): v=%v ok=%v err=%v", caso.clave, v, ok, err)
		}
	}

	v, ok, err := m.intentaStub([]string{"Ljava/util/HashMap;"}, "keySet", "()Ljava/util/Set;", nil, mapa)
	if err != nil || !ok || v == nil {
		t.Fatalf("keySet: v=%v ok=%v err=%v", v, ok, err)
	}
	conjunto := esperaObjetoDeStub(t, v, "Ljava/util/Set;")
	for _, caso := range []struct {
		clave    string
		esperado int32
	}{
		{"a", int32(1)}, {"z", int32(1)}, {"ausente", int32(0)},
	} {
		v, ok, err := m.intentaStub([]string{"Ljava/util/Set;"}, "contains",
			"(Ljava/lang/Object;)Z", []any{cadenaObjeto(caso.clave)}, conjunto)
		if err != nil || !ok || v != caso.esperado {
			t.Errorf("contains(%q): v=%v ok=%v err=%v", caso.clave, v, ok, err)
		}
	}

	// el Map de getAll sale de la misma fuente: contiene solo lo que el almacén guardó.
	actividad := objDe("Lcom/termux/app/TermuxActivity;")
	m.actividad = actividad
	prefs, ok, err := m.intentaStub([]string{"Lcom/termux/app/TermuxActivity;", "Landroid/content/Context;"},
		"getSharedPreferences", "(Ljava/lang/String;I)Landroid/content/SharedPreferences;",
		[]any{cadenaObjeto("sonda"), int32(0)}, actividad)
	if err != nil || !ok || prefs == nil {
		t.Fatalf("getSharedPreferences: v=%v ok=%v err=%v", prefs, ok, err)
	}
	m.almacen.de("sonda")["guardada"] = "valor"
	v, ok, err = m.intentaStub([]string{"Landroid/content/SharedPreferences;"}, "getAll",
		"()Ljava/util/Map;", nil, prefs)
	if err != nil || !ok || v == nil {
		t.Fatalf("getAll: v=%v ok=%v err=%v", v, ok, err)
	}
	mapaPreferencias := v
	if v, ok, err := m.intentaStub([]string{"Ljava/util/Map;"}, "containsKey",
		"(Ljava/lang/Object;)Z", []any{cadenaObjeto("guardada")}, mapaPreferencias); err != nil || !ok ||
		v != int32(1) {
		t.Errorf("containsKey sobre getAll: v=%v ok=%v err=%v", v, ok, err)
	}
}
