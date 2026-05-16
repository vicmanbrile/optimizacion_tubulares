package main

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type BarraResultado struct {
	Numero      int
	Total       float64
	Cortes      []float64
	CortesTexto string
	Usado       float64
	Sobrante    float64
}

type RetalInput struct {
	Medida   string
	Cantidad string
	Tipo     string
}

type PiezaInput struct {
	Medida   string
	Cantidad string
}

type DatosPagina struct {
	StockEstandar string
	Retales       []RetalInput
	Piezas        []PiezaInput
	Resultados    []BarraResultado
}

var htmlTemplate = `
<!DOCTYPE html>
<html lang="es">
<head>
    <meta charset="UTF-8">
    <title>Optimización de Corte de Tubulares</title>
    <script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
</head>
<body class="bg-gray-100 text-gray-800 font-sans antialiased p-6">
    <div class="max-w-4xl mx-auto bg-white p-8 rounded-xl shadow-md">
        <h1 class="text-2xl font-bold mb-6 text-gray-900 border-b pb-4">Calculador de Optimización de Corte</h1>
        
        <form method="POST" class="space-y-8">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
                <div class="space-y-6">
                    <div>
                        <h2 class="text-sm font-bold text-gray-700 mb-3 uppercase tracking-wider">Tubular Estándar (Ilimitado)</h2>
                        <input type="number" step="any" name="stock_estandar" placeholder="Medida Estándar (cm)" class="w-full p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required value="{{.StockEstandar}}">
                    </div>

                    <div>
                        <h2 class="text-sm font-bold text-gray-700 mb-3 uppercase tracking-wider">Retales / Tramos Extras</h2>
                        <div id="contenedor-stock" class="space-y-2">
                            {{range .Retales}}
                            <div class="flex gap-2 fila-input items-center">
                                <input type="number" step="any" name="stock_medida" placeholder="{{if eq .Tipo "y"}}Cord.Y Máquina (mm){{else}}Medida (cm){{end}}" class="w-1/2 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required value="{{.Medida}}">
                                <input type="number" name="stock_cantidad" placeholder="Cant" class="w-1/4 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required value="{{.Cantidad}}">
                                <input type="hidden" name="stock_tipo" value="{{.Tipo}}">
                                <label class="flex items-center gap-1 text-xs text-gray-600 select-none">
                                    <input type="checkbox" onchange="toggleY(this)" {{if eq .Tipo "y"}}checked{{end}} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"> Y
                                </label>
                                <button type="button" onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 font-bold px-2 text-sm transition duration-150">✕</button>
                            </div>
                            {{end}}
                        </div>
                        <button type="button" onclick="agregarFilaStock()" class="mt-3 text-sm font-semibold text-blue-600 hover:text-blue-800 flex items-center gap-1">+ Añadir Retal Extra</button>
                    </div>
                </div>

                <div>
                    <h2 class="text-sm font-bold text-gray-700 mb-3 uppercase tracking-wider">Tramos Requeridos (Piezas)</h2>
                    <div id="contenedor-piezas" class="space-y-2">
                        {{range .Piezas}}
                        <div class="flex gap-2 fila-input items-center">
                            <input type="number" step="any" name="piezas_medida" placeholder="Medida (cm)" class="w-2/3 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required value="{{.Medida}}">
                            <input type="number" name="piezas_cantidad" placeholder="Cantidad" class="w-1/3 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required value="{{.Cantidad}}">
                            <button type="button" onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 font-bold px-2 text-sm transition duration-150">✕</button>
                        </div>
                        {{end}}
                    </div>
                    <button type="button" onclick="agregarFilaPieza()" class="mt-3 text-sm font-semibold text-blue-600 hover:text-blue-800 flex items-center gap-1">+ Añadir Medida de Pieza</button>
                </div>
            </div>

            <button type="submit" class="w-full bg-blue-600 hover:bg-blue-700 text-white font-bold py-3 px-4 rounded-lg transition duration-200 shadow-sm mt-4">Calcular Distribución</button>
        </form>

        {{if .Resultados}}
        <div class="mt-8 space-y-6">
            <h2 class="text-xl font-bold text-gray-900">Esquema de Corte Sugerido</h2>
            <div class="overflow-x-auto">
                <table class="w-full text-left border-collapse">
                    <thead>
                        <tr class="bg-gray-200 text-gray-700 text-sm font-semibold">
                            <th class="p-3 rounded-l-lg">Barra / Tramo</th>
                            <th class="p-3">Tamaño Inicial</th>
                            <th class="p-3">Cortes a realizar</th>
                            <th class="p-3">Longitud Usada</th>
                            <th class="p-3 rounded-r-lg text-right">Sobrante Final</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 text-sm">
                        {{range .Resultados}}
                        <tr class="hover:bg-gray-50">
                            <td class="p-3 font-medium text-gray-900">#{{.Numero}}</td>
                            <td class="p-3 text-gray-600">{{dosDec .Total}} cm</td>
                            <td class="p-3">
                                <span class="bg-blue-50 text-blue-700 px-2 py-1 rounded font-mono font-semibold">
                                    {{.CortesTexto}}
                                </span>
                            </td>
                            <td class="p-3 text-gray-600">{{dosDec .Usado}} cm</td>
                            <td class="p-3 text-right font-bold {{if font_sobrante .Sobrante}}text-green-600{{else}}text-gray-500{{end}}">{{dosDec .Sobrante}} cm</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}
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
            var nuevaFila = document.createElement('div');
            nuevaFila.className = 'flex gap-2 fila-input items-center';
            nuevaFila.innerHTML = '<input type="number" step="any" name="stock_medida" placeholder="Medida (cm)" class="w-1/2 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required> <input type="number" name="stock_cantidad" placeholder="Cant" class="w-1/4 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required> <input type="hidden" name="stock_tipo" value="cm"> <label class="flex items-center gap-1 text-xs text-gray-600 select-none"><input type="checkbox" onchange="toggleY(this)" class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"> Y</label> <button type="button" onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 font-bold px-2 text-sm transition duration-150">✕</button>';
            contenedor.appendChild(nuevaFila);
        }

        function agregarFilaPieza() {
            var contenedor = document.getElementById('contenedor-piezas');
            var nuevaFila = document.createElement('div');
            nuevaFila.className = 'flex gap-2 fila-input items-center';
            nuevaFila.innerHTML = '<input type="number" step="any" name="piezas_medida" placeholder="Medida (cm)" class="w-2/3 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required> <input type="number" name="piezas_cantidad" placeholder="Cantidad" class="w-1/3 p-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:outline-none text-sm" required> <button type="button" onclick="this.parentElement.remove()" class="text-red-500 hover:text-red-700 font-bold px-2 text-sm transition duration-150">✕</button>';
            contenedor.appendChild(nuevaFila);
        }
    </script>
</body>
</html>
`

func optimizarCortes(w http.ResponseWriter, r *http.Request) {
	tmpl := template.New("calculadora")
	tmpl = tmpl.Funcs(template.FuncMap{
		"font_sobrante": func(sobrante float64) bool {
			return sobrante > 1.5
		},
		"dosDec": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
	})
	tmpl, _ = tmpl.Parse(htmlTemplate)

	if r.Method != http.MethodPost {
		inicial := DatosPagina{
			StockEstandar: "565",
			Piezas: []PiezaInput{},
		}
		tmpl.Execute(w, inicial)
		return
	}

	r.ParseForm()

	stockEstandarStr := r.FormValue("stock_estandar")
	stockEstandar, errE := strconv.ParseFloat(stockEstandarStr, 64)
	if errE != nil {
		stockEstandar = 56.5
	}

	stockMedidas := r.Form["stock_medida"]
	stockCantidades := r.Form["stock_cantidad"]
	stockTipos := r.Form["stock_tipo"]
	piezasMedidas := r.Form["piezas_medida"]
	piezasCantidades := r.Form["piezas_cantidad"]

	var retalesInput []RetalInput
	for i := range stockMedidas {
		retalesInput = append(retalesInput, RetalInput{Medida: stockMedidas[i], Cantidad: stockCantidades[i], Tipo: stockTipos[i]})
	}

	var piezasInput []PiezaInput
	for i := range piezasMedidas {
		piezasInput = append(piezasInput, PiezaInput{Medida: piezasMedidas[i], Cantidad: piezasCantidades[i]})
	}

	var resultados []BarraResultado
	for i := range stockMedidas {
		medida, err1 := strconv.ParseFloat(stockMedidas[i], 64)
		cantidad, err2 := strconv.Atoi(stockCantidades[i])
		if err1 == nil && err2 == nil {
			if stockTipos[i] == "y" {
				medida = (6220.80 - medida) / 10.0
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
	for i := range piezasMedidas {
		medida, err1 := strconv.ParseFloat(piezasMedidas[i], 64)
		cantidad, err2 := strconv.Atoi(piezasCantidades[i])
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

	tmpl.Execute(w, DatosPagina{
		StockEstandar: stockEstandarStr,
		Retales:       retalesInput,
		Piezas:        piezasInput,
		Resultados:    resultadosFiltrados,
	})
}

func main() {
	http.HandleFunc("/", optimizarCortes)
	http.ListenAndServe(":8080", nil)
}