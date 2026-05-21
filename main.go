package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed tailwind.js
var tailwindJS string

type BarraResultado struct {
	Numero      int
	Total       float64
	Cortes      []float64
	CortesTexto string
	Usado       float64
	Sobrante    float64
}

type RetalInput struct {
	Medida   string `json:"medida"`
	Cantidad string `json:"cantidad"`
	Tipo     string `json:"tipo"`
}

type PiezaInput struct {
	Medida   string `json:"medida"`
	Cantidad string `json:"cantidad"`
}

type ProyectoData struct {
	StockEstandar string       `json:"stock_estandar"`
	Retales       []RetalInput `json:"retales"`
	Piezas        []PiezaInput `json:"piezas"`
}

type Proyecto struct {
	ID         int
	Nombre     string
	Datos      ProyectoData
	Completado bool
}

type DatosPagina struct {
	Proyectos      []Proyecto
	ProyectoActual Proyecto
	Resultados     []BarraResultado
}


var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./cortes.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `CREATE TABLE IF NOT EXISTS proyectos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nombre TEXT,
		datos TEXT,
		completado BOOLEAN DEFAULT 0
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}
}

func calcularDistribucion(datos ProyectoData) []BarraResultado {
	stockEstandar, errE := strconv.ParseFloat(datos.StockEstandar, 64)
	if errE != nil || stockEstandar <= 0 {
		stockEstandar = 56.5
	}

	var resultados []BarraResultado

	for _, r := range datos.Retales {
		medida, err1 := strconv.ParseFloat(r.Medida, 64)
		cantidad, err2 := strconv.Atoi(r.Cantidad)
		if err1 == nil && err2 == nil {
			if r.Tipo == "y" {
				medida = (6228.70 - medida) / 10.0
			}
			if medida > 0 {
				for c := 0; c < cantidad; c++ {
					resultados = append(resultados, BarraResultado{
						Total:  medida,
						Cortes: []float64{},
					})
				}
			}
		}
	}

	sort.Slice(resultados, func(i, j int) bool {
		return resultados[i].Total < resultados[j].Total
	})

	var listaTramos []float64
	for _, p := range datos.Piezas {
		medida, err1 := strconv.ParseFloat(p.Medida, 64)
		cantidad, err2 := strconv.Atoi(p.Cantidad)
		if err1 == nil && err2 == nil {
			for c := 0; c < cantidad; c++ {
				listaTramos = append(listaTramos, medida)
			}
		}
	}

	sort.Float64s(listaTramos)
	for i, j := 0, len(listaTramos)-1; i < j; i, j = i+1, j-1 {
		listaTramos[i], listaTramos[j] = listaTramos[j], listaTramos[i]
	}

	for _, corte := range listaTramos {
		asignado := false
		for i := range resultados {
			if resultados[i].Usado+corte <= resultados[i].Total {
				resultados[i].Cortes = append(resultados[i].Cortes, corte)
				resultados[i].Usado += corte
				asignado = true
				break
			}
		}
		if !asignado {
			nuevaBarra := BarraResultado{
				Total:  stockEstandar,
				Cortes: []float64{corte},
				Usado:  corte,
			}
			resultados = append(resultados, nuevaBarra)
		}
	}

	var resultadosFiltrados []BarraResultado
	cont := 1
	for i := range resultados {
		if resultados[i].Usado > 0 {
			var parts []string
			if len(resultados[i].Cortes) > 0 {
				currentVal := resultados[i].Cortes[0]
				count := 1
				for j := 1; j < len(resultados[i].Cortes); j++ {
					if resultados[i].Cortes[j] == currentVal {
						count++
					} else {
						parts = append(parts, fmt.Sprintf("%.2f(%d)", currentVal, count))
						currentVal = resultados[i].Cortes[j]
						count = 1
					}
				}
				parts = append(parts, fmt.Sprintf("%.2f(%d)", currentVal, count))
			}
			resultados[i].CortesTexto = strings.Join(parts, ", ")
			resultados[i].Numero = cont
			resultados[i].Sobrante = resultados[i].Total - resultados[i].Usado
			resultadosFiltrados = append(resultadosFiltrados, resultados[i])
			cont++
		}
	}

	return resultadosFiltrados
}


func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.New("calculadora").Funcs(template.FuncMap{
		"font_sobrante": func(sobrante float64) bool { return sobrante > 1.5 },
		"dosDec":        func(f float64) string { return fmt.Sprintf("%.2f", f) },
	})
	tmpl, _ = tmpl.Parse(htmlTemplate)

	rows, err := db.Query("SELECT id, nombre, completado FROM proyectos ORDER BY id DESC")
	var proyectos []Proyecto
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p Proyecto
			rows.Scan(&p.ID, &p.Nombre, &p.Completado)
			proyectos = append(proyectos, p)
		}
	}

	dataPagina := DatosPagina{
		Proyectos: proyectos,
		ProyectoActual: Proyecto{
			Nombre: "Nuevo Proyecto",
			Datos: ProyectoData{
				StockEstandar: "565",
				Piezas:        []PiezaInput{},
				Retales:       []RetalInput{},
			},
		},
	}

	idStr := r.URL.Query().Get("id")
	if idStr != "" {
		id, _ := strconv.Atoi(idStr)
		var datosJSON string
		err := db.QueryRow("SELECT id, nombre, datos, completado FROM proyectos WHERE id = ?", id).
			Scan(&dataPagina.ProyectoActual.ID, &dataPagina.ProyectoActual.Nombre, &datosJSON, &dataPagina.ProyectoActual.Completado)
		
		if err == nil {
			json.Unmarshal([]byte(datosJSON), &dataPagina.ProyectoActual.Datos)
			// Calculamos solo si hay datos guardados
			if len(dataPagina.ProyectoActual.Datos.Piezas) > 0 {
				dataPagina.Resultados = calcularDistribucion(dataPagina.ProyectoActual.Datos)
			}
		}
	}

	tmpl.Execute(w, dataPagina)
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	r.ParseForm()

	nombre := r.FormValue("nombre")
	if strings.TrimSpace(nombre) == "" {
		nombre = "Proyecto sin nombre"
	}

	datos := ProyectoData{
		StockEstandar: r.FormValue("stock_estandar"),
		Retales:       []RetalInput{},
		Piezas:        []PiezaInput{},
	}

	sMed := r.Form["stock_medida"]
	sCan := r.Form["stock_cantidad"]
	sTip := r.Form["stock_tipo"]
	for i := range sMed {
		if sMed[i] != "" && sCan[i] != "" {
			datos.Retales = append(datos.Retales, RetalInput{Medida: sMed[i], Cantidad: sCan[i], Tipo: sTip[i]})
		}
	}

	pMed := r.Form["piezas_medida"]
	pCan := r.Form["piezas_cantidad"]
	for i := range pMed {
		if pMed[i] != "" && pCan[i] != "" {
			datos.Piezas = append(datos.Piezas, PiezaInput{Medida: pMed[i], Cantidad: pCan[i]})
		}
	}

	datosJSON, _ := json.Marshal(datos)
	idStr := r.FormValue("id")
	
	if idStr == "" || idStr == "0" {
		result, _ := db.Exec("INSERT INTO proyectos (nombre, datos) VALUES (?, ?)", nombre, string(datosJSON))
		newID, _ := result.LastInsertId()
		http.Redirect(w, r, fmt.Sprintf("/?id=%d", newID), http.StatusSeeOther)
	} else {
		db.Exec("UPDATE proyectos SET nombre = ?, datos = ? WHERE id = ?", nombre, string(datosJSON), idStr)
		http.Redirect(w, r, fmt.Sprintf("/?id=%s", idStr), http.StatusSeeOther)
	}
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id != "" {
		db.Exec("DELETE FROM proyectos WHERE id = ?", id)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func toggleCompleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id != "" && id != "0" {
		db.Exec("UPDATE proyectos SET completado = NOT completado WHERE id = ?", id)
	}
	http.Redirect(w, r, fmt.Sprintf("/?id=%s", id), http.StatusSeeOther)
}

func serveJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(tailwindJS))
}

func main() {
	initDB()

	http.HandleFunc("/tailwind.js", serveJS)
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/save", saveHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/toggle", toggleCompleteHandler)

	fmt.Println("Servidor con Base de Datos iniciado en http://localhost:80")
	http.ListenAndServe(":80", nil)
}


var htmlTemplate = `
<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <title>{{if .ProyectoActual.Nombre}}{{.ProyectoActual.Nombre}}{{else}}Nuevo Proyecto{{end}} - Calculador</title>
    <script src="/tailwind.js"></script>
</head>
<body class="bg-gray-100 text-gray-800 font-sans antialiased h-screen overflow-hidden flex">

    <!-- BARRA LATERAL -->
    <div class="w-72 bg-gray-900 text-white flex flex-col shadow-lg z-10">
        <div class="p-6 border-b border-gray-700">
            <h2 class="text-xl font-bold tracking-wider">Historial</h2>
            <p class="text-gray-400 text-sm mt-1">Proyectos guardados</p>
        </div>
        
        <div class="flex-1 overflow-y-auto p-4 space-y-3 scrollbar-thin scrollbar-thumb-gray-700">
            {{range .Proyectos}}
            <div class="flex justify-between items-center group bg-gray-800 hover:bg-gray-700 rounded-lg p-3 transition duration-200 border-l-4 {{if .Completado}}border-green-500{{else}}border-blue-500{{end}}">
                <a href="/?id={{.ID}}" class="flex-1 truncate block {{if .Completado}}line-through text-gray-400{{else}}text-gray-100{{end}}">
                    {{.Nombre}}
                </a>
                <a href="/delete?id={{.ID}}" onclick="return confirm('¿Borrar proyecto?')" class="text-gray-500 hover:text-red-400 opacity-0 group-hover:opacity-100 transition px-2">
                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path></svg>
                </a>
            </div>
            {{end}}
        </div>

        <div class="p-4 border-t border-gray-700">
            <a href="/" class="flex items-center justify-center w-full bg-blue-600 hover:bg-blue-500 text-white font-bold py-3 px-4 rounded-lg transition duration-200 shadow-md gap-2">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
                Nuevo Proyecto
            </a>
        </div>
    </div>

    <!-- CONTENIDO PRINCIPAL -->
    <div class="flex-1 overflow-y-auto p-8 relative bg-gray-50">
        <div class="max-w-5xl mx-auto bg-white p-8 rounded-xl shadow-sm border border-gray-200">
            
            <form method="POST" action="/save">
                <!-- ID OCULTO -->
                <input type="hidden" name="id" value="{{.ProyectoActual.ID}}">
                
                <!-- NOMBRE EDITABLE -->
                <div class="mb-8 border-b pb-4">
                    <input type="text" name="nombre" value="{{.ProyectoActual.Nombre}}" placeholder="Escribe el nombre del proyecto..." 
                           class="text-3xl font-bold text-gray-900 bg-transparent border-0 border-b-2 border-transparent hover:border-gray-200 focus:border-blue-500 focus:ring-0 w-full px-0 py-2 outline-none transition">
                </div>
                
                <div class="grid grid-cols-1 lg:grid-cols-2 gap-10">
                    <div class="space-y-8">
                        <!-- TUBULAR ESTANDAR -->
                        <div class="bg-gray-50 p-5 rounded-lg border border-gray-100">
                            <h2 class="text-xs font-bold text-gray-500 mb-3 uppercase tracking-wider">Tubular Estándar (Ilimitado)</h2>
                            <input type="number" step="any" name="stock_estandar" placeholder="Medida Estándar (cm)" class="w-full p-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition shadow-sm" required value="{{.ProyectoActual.Datos.StockEstandar}}">
                        </div>

                        <!-- RETALES -->
                        <div class="bg-blue-50/50 p-5 rounded-lg border border-blue-100">
                            <h2 class="text-xs font-bold text-blue-800 mb-3 uppercase tracking-wider">Retales / Tramos Extras</h2>
                            <div id="contenedor-stock" class="space-y-3">
                                {{range .ProyectoActual.Datos.Retales}}
                                <div class="flex gap-2 fila-input items-center bg-white p-2 rounded border border-gray-200 shadow-sm">
                                    <input type="number" step="any" name="stock_medida" placeholder="{{if eq .Tipo "y"}}Cord.Y Máquina (mm){{else}}Medida (cm){{end}}" class="w-1/2 p-2 bg-transparent outline-none border-b focus:border-blue-500 text-sm" required value="{{.Medida}}">
                                    <input type="number" name="stock_cantidad" placeholder="Cant" class="w-1/4 p-2 bg-transparent outline-none border-b focus:border-blue-500 text-sm text-center" required value="{{.Cantidad}}">
                                    <input type="hidden" name="stock_tipo" value="{{.Tipo}}">
                                    <label class="flex items-center gap-1 text-xs text-gray-600 font-medium cursor-pointer">
                                        <input type="checkbox" onchange="toggleY(this)" {{if eq .Tipo "y"}}checked{{end}} class="w-4 h-4 text-blue-600 rounded border-gray-300"> Y
                                    </label>
                                    <button type="button" onclick="this.parentElement.remove()" class="text-red-400 hover:text-red-600 ml-auto px-2">✕</button>
                                </div>
                                {{end}}
                            </div>
                            <button type="button" onclick="agregarFilaStock()" class="mt-4 text-sm font-bold text-blue-600 hover:text-blue-800 flex items-center gap-1 bg-white px-3 py-1.5 rounded shadow-sm border border-blue-200 hover:border-blue-300 transition">+ Añadir Retal</button>
                        </div>
                    </div>

                    <!-- PIEZAS -->
                    <div class="bg-orange-50/50 p-5 rounded-lg border border-orange-100">
                        <h2 class="text-xs font-bold text-orange-800 mb-3 uppercase tracking-wider">Tramos Requeridos (Piezas a cortar)</h2>
                        <div id="contenedor-piezas" class="space-y-3">
                            {{range .ProyectoActual.Datos.Piezas}}
                            <div class="flex gap-2 fila-input items-center bg-white p-2 rounded border border-gray-200 shadow-sm">
                                <input type="number" step="any" name="piezas_medida" placeholder="Medida (cm)" class="w-2/3 p-2 bg-transparent outline-none border-b focus:border-orange-500 text-sm" required value="{{.Medida}}">
                                <input type="number" name="piezas_cantidad" placeholder="Cantidad" class="w-1/3 p-2 bg-transparent outline-none border-b focus:border-orange-500 text-sm text-center" required value="{{.Cantidad}}">
                                <button type="button" onclick="this.parentElement.remove()" class="text-red-400 hover:text-red-600 px-2">✕</button>
                            </div>
                            {{end}}
                        </div>
                        <button type="button" onclick="agregarFilaPieza()" class="mt-4 text-sm font-bold text-orange-600 hover:text-orange-800 flex items-center gap-1 bg-white px-3 py-1.5 rounded shadow-sm border border-orange-200 hover:border-orange-300 transition">+ Añadir Pieza</button>
                    </div>
                </div>

                <div class="mt-8 pt-6 border-t flex justify-end">
                    <button type="submit" class="bg-gray-900 hover:bg-black text-white font-bold py-3 px-8 rounded-lg transition duration-200 shadow-md text-lg flex items-center gap-2">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"></path></svg>
                        Guardar y Calcular
                    </button>
                </div>
            </form>

            <!-- RESULTADOS -->
            {{if .Resultados}}
            <div class="mt-12 bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
                <div class="bg-gray-50 p-6 border-b flex justify-between items-center">
                    <h2 class="text-xl font-bold text-gray-900">Esquema de Corte Optimizado</h2>
                    
                    <!-- Boton Completado -->
                    <form method="POST" action="/toggle">
                        <input type="hidden" name="id" value="{{.ProyectoActual.ID}}">
                        {{if .ProyectoActual.Completado}}
                            <button type="submit" class="bg-yellow-100 hover:bg-yellow-200 text-yellow-800 border border-yellow-300 font-bold py-2 px-4 rounded-lg transition duration-200 shadow-sm text-sm flex items-center gap-2">
                                ↺ Reabrir Proyecto
                            </button>
                        {{else}}
                            <button type="submit" class="bg-green-600 hover:bg-green-700 text-white font-bold py-2 px-4 rounded-lg transition duration-200 shadow-sm text-sm flex items-center gap-2">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
                                Marcar como Completado
                            </button>
                        {{end}}
                    </form>
                </div>
                
                <div class="overflow-x-auto p-0">
                    <table class="w-full text-left border-collapse">
                        <thead>
                            <tr class="bg-gray-100 text-gray-600 text-xs uppercase tracking-wider">
                                <th class="p-4 border-b">Barra</th>
                                <th class="p-4 border-b">Tamaño</th>
                                <th class="p-4 border-b">Distribución de Cortes</th>
                                <th class="p-4 border-b">Usado</th>
                                <th class="p-4 border-b text-right">Sobrante</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-100 text-sm">
                            {{range .Resultados}}
                            <tr class="hover:bg-blue-50/30 transition">
                                <td class="p-4 font-bold text-gray-900">#{{.Numero}}</td>
                                <td class="p-4 text-gray-600 font-medium">{{dosDec .Total}}</td>
                                <td class="p-4">
                                    <span class="bg-gray-100 text-gray-800 border border-gray-200 px-3 py-1.5 rounded font-mono font-semibold text-xs tracking-wide">
                                        {{.CortesTexto}}
                                    </span>
                                </td>
                                <td class="p-4 text-gray-600">{{dosDec .Usado}}</td>
                                <td class="p-4 text-right font-bold {{if font_sobrante .Sobrante}}text-green-600{{else}}text-red-400{{end}}">
                                    {{dosDec .Sobrante}}
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
            {{end}}
        </div>
    </div>

    <script>
        function toggleY(cb) {
            var fila = cb.closest('.fila-input');
            var inputMedida = fila.querySelector('input[name="stock_medida"]');
            var inputTipo = fila.querySelector('input[name="stock_tipo"]');
            if (cb.checked) {
                inputTipo.value = 'y';
                inputMedida.placeholder = 'Y Máquina (mm)';
            } else {
                inputTipo.value = 'cm';
                inputMedida.placeholder = 'Medida (cm)';
            }
        }

        function agregarFilaStock() {
            var contenedor = document.getElementById('contenedor-stock');
            var html = '<div class="flex gap-2 fila-input items-center bg-white p-2 rounded border border-gray-200 shadow-sm"><input type="number" step="any" name="stock_medida" placeholder="Medida (cm)" class="w-1/2 p-2 bg-transparent outline-none border-b focus:border-blue-500 text-sm" required> <input type="number" name="stock_cantidad" placeholder="Cant" class="w-1/4 p-2 bg-transparent outline-none border-b focus:border-blue-500 text-sm text-center" required> <input type="hidden" name="stock_tipo" value="cm"> <label class="flex items-center gap-1 text-xs text-gray-600 font-medium cursor-pointer"><input type="checkbox" onchange="toggleY(this)" class="w-4 h-4 text-blue-600 rounded border-gray-300"> Y</label> <button type="button" onclick="this.parentElement.remove()" class="text-red-400 hover:text-red-600 ml-auto px-2">✕</button></div>';
            contenedor.insertAdjacentHTML('beforeend', html);
        }

        function agregarFilaPieza() {
            var contenedor = document.getElementById('contenedor-piezas');
            var html = '<div class="flex gap-2 fila-input items-center bg-white p-2 rounded border border-gray-200 shadow-sm"><input type="number" step="any" name="piezas_medida" placeholder="Medida (cm)" class="w-2/3 p-2 bg-transparent outline-none border-b focus:border-orange-500 text-sm" required> <input type="number" name="piezas_cantidad" placeholder="Cantidad" class="w-1/3 p-2 bg-transparent outline-none border-b focus:border-orange-500 text-sm text-center" required> <button type="button" onclick="this.parentElement.remove()" class="text-red-400 hover:text-red-600 px-2">✕</button></div>';
            contenedor.insertAdjacentHTML('beforeend', html);
        }
    </script>
</body>
</html>
`