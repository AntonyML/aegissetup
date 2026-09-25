// © Antony Monge López — Costa Rica — Céd. 604700548

using System;
using System.Collections;
using System.Collections.Generic;
using System.Data.Odbc;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.RegularExpressions;
using System.Windows.Forms;

namespace Aegis.ReportHost
{
    internal static class Program
    {
        [STAThread]
        private static int Main(string[] args)
        {
            var options = Options.Parse(args);
            if (options.Error != null)
            {
                Protocol.Error("process_start", "invalid_arguments", options.Error, null);
                return 2;
            }

            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            return new CrystalSession(options).Run();
        }
    }

    internal sealed class Options
    {
        internal string Mode;
        internal string Report;
        internal string Dsn;
        internal string Database;
        internal string User;
        internal string Output;
        internal bool WindowsAuth;
        internal string Error;

        internal static Options Parse(string[] args)
        {
            var o = new Options { Mode = "check", Dsn = "SIDC_SQL", Database = "SIDC" };
            for (var i = 0; i < args.Length; i++)
            {
                var key = args[i];
                var value = i + 1 < args.Length ? args[++i] : null;
                switch (key)
                {
                    case "--mode": o.Mode = value; break;
                    case "--report": o.Report = value; break;
                    case "--dsn": o.Dsn = value; break;
                    case "--database": o.Database = value; break;
                    case "--user": o.User = value; break;
                    case "--output": o.Output = value; break;
                    default:
                        o.Error = "opción desconocida: " + key;
                        return o;
                }
                if (value == null)
                {
                    o.Error = "falta valor para " + key;
                    return o;
                }
            }
            if (o.Mode != "check" && o.Mode != "view" && o.Mode != "export-pdf") o.Error = "modo inválido: " + o.Mode;
            if (String.IsNullOrWhiteSpace(o.Report)) o.Error = "falta --report";
            if (String.IsNullOrWhiteSpace(o.Dsn)) o.Error = "falta --dsn";
            if (String.IsNullOrWhiteSpace(o.Database)) o.Error = "falta --database";
            if (o.Mode == "export-pdf" && String.IsNullOrWhiteSpace(o.Output)) o.Error = "falta --output";
            o.WindowsAuth = Environment.GetEnvironmentVariable("AEGIS_REPORT_WIN_AUTH") == "1";
            return o;
        }
    }

    internal static class Protocol
    {
        private static readonly Stopwatch Clock = Stopwatch.StartNew();

        internal static void Stage(string stage, bool success, string errorCode, string hresult, string crystalError, IDictionary<string, object> details)
        {
            var item = new Dictionary<string, object>();
            item["success"] = success;
            item["stage"] = stage;
            item["errorCode"] = errorCode;
            item["hresult"] = hresult;
            item["crystalError"] = crystalError;
            item["elapsedMs"] = Clock.ElapsedMilliseconds;
            if (details != null) item["details"] = details;
            Console.WriteLine(Json(item));
            Console.Out.Flush();
        }

        internal static void Error(string stage, string errorCode, string message, Exception error)
        {
            var hresult = error == null ? null : FormatHResult(error.HResult);
            var details = new Dictionary<string, object>();
            details["errorType"] = error == null ? null : error.GetType().FullName;
            details["stackTrace"] = error == null ? null : Clean(error.StackTrace);
            Stage(stage, false, errorCode, hresult, Clean(message), details);
        }

        private static string Json(IDictionary<string, object> values)
        {
            var parts = new List<string>();
            foreach (var pair in values)
            {
                parts.Add("\"" + Escape(pair.Key) + "\":" + Value(pair.Value));
            }
            return "{" + String.Join(",", parts) + "}";
        }

        private static string Value(object value)
        {
            if (value == null) return "null";
            if (value is bool) return (bool)value ? "true" : "false";
            if (value is long) return value.ToString();
            if (value is IDictionary<string, object>) return Json((IDictionary<string, object>)value);
            return "\"" + Escape(Convert.ToString(value)) + "\"";
        }

        private static string Escape(string value)
        {
            if (value == null) return "";
            return value.Replace("\\", "\\\\").Replace("\"", "\\\"")
                .Replace("\r", "\\r").Replace("\n", "\\n").Replace("\t", "\\t");
        }

        internal static string Clean(string value)
        {
            if (String.IsNullOrEmpty(value)) return value;
            var result = Regex.Replace(value, @"(?i)(PWD|PASSWORD|Passwd|Token|Secret)\s*[=:]\s*[^;\r\n]*", "$1=****");
            return Regex.Replace(result, @"[\r\n]+", " ");
        }

        internal static string FormatHResult(int value)
        {
            return "0x" + value.ToString("X8");
        }
    }

    internal sealed class CrystalSession
    {
        private const string ApplicationProgId = "CrystalRuntime.Application";
        private const string ViewerClsid = "{C4847596-972C-11D0-9567-00A0C9273C2A}";

        private readonly Options _options;
        private object _application;
        private object _report;
        private CrystalViewerControl _viewer;
        private Form _form;
        private string _stage = "process_start";

        internal CrystalSession(Options options)
        {
            _options = options;
        }

        internal int Run()
        {
            Exception failure = null;
            string failureStage = null;
            Protocol.Stage("process_start", true, null, null, null, null);
            try
            {
                CreateApplication();
                OpenReport();
                Authenticate();
                InspectParameters();
                DiscardSavedData();
                ReadRecords();
                if (_options.Mode == "view") ShowViewer();
                if (_options.Mode == "export-pdf") ExportPdf();
            }
            catch (Exception error)
            {
                failure = Unwrap(error);
                failureStage = _stage;
            }

            Exception cleanupFailure = null;
            try
            {
                Cleanup();
            }
            catch (Exception error)
            {
                cleanupFailure = Unwrap(error);
            }

            if (failure != null)
            {
                Protocol.Error(failureStage, ErrorCode(failure), failure.Message, failure);
                return 1;
            }
            if (cleanupFailure != null)
            {
                Protocol.Error("com_cleanup", ErrorCode(cleanupFailure), cleanupFailure.Message, cleanupFailure);
                return 1;
            }
            var exitDetails = new Dictionary<string, object>();
            exitDetails["mode"] = _options.Mode;
            Protocol.Stage("process_exit", true, null, null, null, exitDetails);
            return 0;
        }

        private void CreateApplication()
        {
            _stage = "com_application";
            var type = Type.GetTypeFromProgID(ApplicationProgId, true);
            _application = Activator.CreateInstance(type);
            Protocol.Stage(_stage, true, null, null, null, null);
        }

        private void OpenReport()
        {
            _stage = "open_report";
            _report = ((dynamic)_application).OpenReport(_options.Report);
            if (_report == null) throw new InvalidOperationException("OpenReport devolvió vacío");
            Protocol.Stage(_stage, true, null, null, null, null);
        }

        private void Authenticate()
        {
            _stage = "database_logon";
            var user = _options.WindowsAuth ? "" : (_options.User ?? "");
            var password = _options.WindowsAuth ? "" : (Environment.GetEnvironmentVariable("AEGIS_REPORT_PASSWORD") ?? "");
            dynamic report = _report;
            report.Database.LogOnServer("p2sodbc.dll", _options.Dsn, _options.Database, user, password);
            Protocol.Stage(_stage, true, null, null, null, null);

            var tableIndex = 0;
            foreach (object table in (IEnumerable)report.Database.Tables)
            {
                tableIndex++;
                _stage = "table_logon";
                ((dynamic)table).SetLogOnInfo(_options.Dsn, _options.Database, user, password);
                var tableDetails = new Dictionary<string, object>();
                tableDetails["tableIndex"] = (long)tableIndex;
                try { tableDetails["tableName"] = Protocol.Clean(Convert.ToString(((dynamic)table).Name)); } catch { }
                try
                {
                    var fieldNames = new List<string>();
                    foreach (object field in (IEnumerable)((dynamic)table).Fields)
                    {
                        fieldNames.Add(Protocol.Clean(Convert.ToString(((dynamic)field).Name)));
                    }
                    tableDetails["fieldCount"] = (long)fieldNames.Count;
                    tableDetails["fieldNames"] = String.Join("|", fieldNames.ToArray());
                }
                catch { }
                Protocol.Stage(_stage, true, null, null, null, tableDetails);
            }
            var tableSummary = new Dictionary<string, object>();
            tableSummary["tableCount"] = (long)tableIndex;
            Protocol.Stage("table_logon_complete", true, null, null, null, tableSummary);
        }

        private void ReadRecords()
        {
            _stage = "read_records";
            ((dynamic)_report).ReadRecords();
            var count = 0;
            try { count = (int)((dynamic)_report).Database.Tables.Count; } catch { }
            var subreports = 0;
            try { subreports = (int)((dynamic)_report).Subreports.Count; } catch { }
            var recordDetails = new Dictionary<string, object>();
            recordDetails["tableCount"] = (long)count;
            recordDetails["subreportCount"] = (long)subreports;
            Protocol.Stage(_stage, true, null, null, null, recordDetails);
        }

        private void DiscardSavedData()
        {
            _stage = "discard_saved_data";
            ((dynamic)_report).DiscardSavedData();
            Protocol.Stage(_stage, true, null, null, null, null);
        }

        private void InspectParameters()
        {
            _stage = "parameters";
            var details = new Dictionary<string, object>();
            try
            {
                details["parameterCount"] = (long)(int)((dynamic)_report).ParameterFields.Count;
                details["available"] = true;
            }
            catch
            {
                details["parameterCount"] = 0L;
                details["available"] = false;
            }
            Protocol.Stage(_stage, true, null, null, null, details);
        }

        private void ShowViewer()
        {
            _stage = "viewer_create";
            _form = new Form
            {
                Text = "SIDC — Rpt_Caja_Chica",
                Width = 1200,
                Height = 800,
                StartPosition = FormStartPosition.CenterScreen
            };
            _viewer = new CrystalViewerControl(ViewerClsid) { Dock = DockStyle.Fill };
            ((System.ComponentModel.ISupportInitialize)_viewer).BeginInit();
            _form.Controls.Add(_viewer);
            ((System.ComponentModel.ISupportInitialize)_viewer).EndInit();
            _form.FormClosing += CloseWithoutDisposing;
            Protocol.Stage(_stage, true, null, null, null, null);

            _stage = "set_report_source";
            _viewer.SetReportSource(_report);
            Protocol.Stage(_stage, true, null, null, null, null);

            _stage = "view_report";
            _viewer.ViewReport();
            Protocol.Stage("viewer_ready", true, null, null, null, null);
            Application.Run(_form);
            Protocol.Stage("close", true, null, null, null, null);
        }

        private void ExportPdf()
        {
            _stage = "export_prepare";
            var output = Path.GetFullPath(_options.Output);
            var outputDirectory = Path.GetDirectoryName(output);
            if (String.IsNullOrWhiteSpace(outputDirectory) || !Directory.Exists(outputDirectory))
            {
                throw new DirectoryNotFoundException("No existe la carpeta de salida del PDF: " + outputDirectory);
            }
            if (File.Exists(output))
            {
                throw new IOException("El archivo de salida ya existe: " + output);
            }
            Protocol.Stage(_stage, true, null, null, null, null);

            var tempRoot = Path.Combine(Path.GetTempPath(), "Aegis.ReportHost", Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(tempRoot);
            try
            {
                _stage = "modern_data_query";
                var htmlPath = Path.Combine(tempRoot, "report.html");
                var data = LoadCajaChicaData();
                var queryDetails = new Dictionary<string, object>();
                queryDetails["detailCount"] = (long)data.Details.Count;
                Protocol.Stage(_stage, true, null, null, null, queryDetails);

                _stage = "modern_html";
                BuildCajaChicaHtml(htmlPath, data);
                var htmlDetails = new Dictionary<string, object>();
                htmlDetails["format"] = "HTML5";
                htmlDetails["renderer"] = "Chrome headless";
                Protocol.Stage(_stage, true, null, null, null, htmlDetails);

                _stage = "pdf_render";
                RenderHtmlToPdf(htmlPath, output, tempRoot);
                var pdfDetails = new Dictionary<string, object>();
                pdfDetails["outputPath"] = output;
                pdfDetails["bytes"] = (long)new FileInfo(output).Length;
                Protocol.Stage("pdf_ready", true, null, null, null, pdfDetails);
            }
            finally
            {
                TryDeleteDirectory(tempRoot);
            }
        }

        private CajaChicaData LoadCajaChicaData()
        {
            var data = new CajaChicaData();
            var connectionString = BuildOdbcConnectionString();
            using (var connection = new OdbcConnection(connectionString))
            {
                connection.Open();
                if (!LoadHeader(connection, data, true))
                {
                    if (!LoadHeader(connection, data, false))
                    {
                        throw new InvalidOperationException("No se encontró una caja chica para generar el PDF");
                    }
                }
                LoadDetails(connection, data);
            }
            return data;
        }

        private string BuildOdbcConnectionString()
        {
            var builder = new OdbcConnectionStringBuilder();
            builder.Dsn = _options.Dsn;
            builder["DATABASE"] = _options.Database;
            if (_options.WindowsAuth)
            {
                builder["Trusted_Connection"] = "Yes";
            }
            else
            {
                builder["UID"] = _options.User ?? "";
                builder["PWD"] = Environment.GetEnvironmentVariable("AEGIS_REPORT_PASSWORD") ?? "";
            }
            return builder.ConnectionString;
        }

        private static bool LoadHeader(OdbcConnection connection, CajaChicaData data, bool activeOnly)
        {
            var where = activeOnly ? " WHERE ISNULL(p.ACTIVO, 0) = 1" : "";
            var sql = "SELECT TOP 1 c.id, c.Caja_Chica, c.Cheque, c.Fecha_Inicio, c.Fecha_Cierre, "
                + "m.Monto AS MontoEstablecido, p.[AÑO] AS PeriodoAnio, "
                + "r.Responsable, r.Puesto, r.Revision, r.Puesto_ "
                + "FROM CAJA_CHICA c "
                + "LEFT JOIN CAJA_CHICA_MONTOESTAB m ON m.id = c.idMontoEstab "
                + "LEFT JOIN PERIODO p ON p.Id = c.idPeriodo "
                + "LEFT JOIN CAJA_CHICA_RESPONSABLES r ON r.id = c.idResponsable"
                + where + " ORDER BY c.id DESC";
            using (var command = new OdbcCommand(sql, connection))
            using (var reader = command.ExecuteReader())
            {
                if (!reader.Read()) return false;
                data.Header.Id = ReadInt(reader, 0);
                data.Header.Caja = ReadText(reader, 1);
                data.Header.Cheque = ReadText(reader, 2);
                data.Header.FechaInicio = ReadDate(reader, 3);
                data.Header.FechaCierre = ReadDate(reader, 4);
                data.Header.MontoEstablecido = ReadDecimal(reader, 5);
                data.Header.PeriodoAnio = ReadText(reader, 6);
                data.Header.Responsable = ReadText(reader, 7);
                data.Header.Puesto = ReadText(reader, 8);
                data.Header.Revision = ReadText(reader, 9);
                data.Header.PuestoRevision = ReadText(reader, 10);
                return true;
            }
        }

        private static void LoadDetails(OdbcConnection connection, CajaChicaData data)
        {
            const string sql = "SELECT Fecha_Fact, No_Factura, Codigo, Proveedor, Descripcion, Monto "
                + "FROM CAJA_CHICA_DETALLE WHERE id_Caja_Chica = ? ORDER BY Fecha_Fact, id";
            using (var command = new OdbcCommand(sql, connection))
            {
                command.Parameters.Add("id_Caja_Chica", OdbcType.Int).Value = data.Header.Id;
                using (var reader = command.ExecuteReader())
                {
                    while (reader.Read())
                    {
                        data.Details.Add(new CajaChicaDetail
                        {
                            Fecha = ReadDate(reader, 0),
                            Factura = ReadText(reader, 1),
                            Codigo = ReadText(reader, 2),
                            Proveedor = ReadText(reader, 3),
                            Descripcion = ReadText(reader, 4),
                            Monto = ReadDecimal(reader, 5)
                        });
                    }
                }
            }
        }

        private static int ReadInt(OdbcDataReader reader, int ordinal)
        {
            return reader.IsDBNull(ordinal) ? 0 : Convert.ToInt32(reader.GetValue(ordinal), CultureInfo.InvariantCulture);
        }

        private static string ReadText(OdbcDataReader reader, int ordinal)
        {
            return reader.IsDBNull(ordinal) ? "" : Convert.ToString(reader.GetValue(ordinal), CultureInfo.InvariantCulture);
        }

        private static DateTime? ReadDate(OdbcDataReader reader, int ordinal)
        {
            return reader.IsDBNull(ordinal) ? (DateTime?)null : Convert.ToDateTime(reader.GetValue(ordinal), CultureInfo.InvariantCulture);
        }

        private static decimal ReadDecimal(OdbcDataReader reader, int ordinal)
        {
            return reader.IsDBNull(ordinal) ? 0m : Convert.ToDecimal(reader.GetValue(ordinal), CultureInfo.InvariantCulture);
        }

        private void BuildCajaChicaHtml(string htmlPath, CajaChicaData data)
        {
            var html = new StringBuilder();
            html.Append("<!doctype html><html><head><meta charset=\"utf-8\"><style>");
            html.Append("@page{size:Letter;margin:0.35in}body{font-family:Arial,sans-serif;color:#111;font-size:9pt;margin:0}.page{page-break-after:always}.page:last-child{page-break-after:auto}");
            html.Append(".header{display:grid;grid-template-columns:1.2in 1fr 1.2in;align-items:center;border-bottom:2px solid #111;padding-bottom:6px}.logo{max-width:1.1in;max-height:.55in}.title{text-align:center;font-weight:bold}.title h1{font-size:13pt;margin:0}.title h2{font-size:11pt;margin:3px 0}.title h3{font-size:10pt;margin:0}.meta{margin-top:8px;border-collapse:collapse;width:100%}.meta td{border:1px solid #777;padding:4px}.label{font-weight:bold;background:#f2f2f2}.summary{border-collapse:collapse;width:60%;margin-top:12px}.summary td{border:1px solid #777;padding:4px}.summary td:last-child{text-align:right;font-weight:bold}.details{border-collapse:collapse;width:100%;margin-top:12px;table-layout:fixed}.details th,.details td{border:1px solid #777;padding:3px}.details th{background:#e9e9e9}.details .date{width:11%}.details .invoice{width:10%}.details .code{width:13%}.details .supplier{width:19%}.details .description{width:35%}.details .amount{width:12%;text-align:right}.totals{border-collapse:collapse;margin:8px 0 0 auto;width:42%}.totals td{padding:3px}.totals td:last-child{text-align:right;border-bottom:1px dotted #333;font-weight:bold}.signatures{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:28px}.signature{border:1px solid #777;text-align:center;padding:18px 4px 10px;min-height:35px}.small{font-size:8pt}</style></head><body>");
            html.Append("<div class=\"page\">");
            html.Append("<div class=\"header\">");
            var logo = FindLogo();
            if (logo != null) html.Append("<img class=\"logo\" src=\"" + logo + "\">"); else html.Append("<div></div>");
            html.Append("<div class=\"title\"><h1>Sistema Integrado de Controles</h1><h2>DETALLE DE GASTOS DE CAJA CHICA</h2><h3>" + Html(data.Header.PeriodoAnio) + "</h3></div><div></div></div>");
            html.Append("<table class=\"meta\"><tr><td><span class=\"label\">CAJA CHICA No.</span> " + Html(data.Header.Caja) + "</td><td><span class=\"label\">Cheque:</span> " + Html(data.Header.Cheque) + "</td><td><span class=\"label\">Periodo del:</span> " + Html(FormatDate(data.Header.FechaInicio)) + " al " + Html(FormatDate(data.Header.FechaCierre)) + "</td></tr></table>");
            var total = 0m;
            foreach (var detail in data.Details) total += detail.Monto;
            var balance = data.Header.MontoEstablecido - total;
            html.Append("<table class=\"summary\"><tr><td>Monto Establecido en Caja Chica</td><td>" + Money(data.Header.MontoEstablecido) + "</td></tr><tr><td>Monto Reintegro de Caja Chica</td><td>" + Money(total) + "</td></tr><tr><td><b>MONTO ACTUAL EN CAJA CHICA</b></td><td>" + Money(balance) + "</td></tr></table>");
            html.Append("<table class=\"details\"><thead><tr><th class=\"date\">Fecha</th><th class=\"invoice\">No. Factura</th><th class=\"code\">Código</th><th class=\"supplier\">Proveedor</th><th class=\"description\">Descripción</th><th class=\"amount\">Monto</th></tr></thead><tbody>");
            foreach (var detail in data.Details)
            {
                html.Append("<tr><td>" + Html(FormatDate(detail.Fecha)) + "</td><td>" + Html(detail.Factura) + "</td><td>" + Html(detail.Codigo) + "</td><td>" + Html(detail.Proveedor) + "</td><td>" + Html(detail.Descripcion) + "</td><td class=\"amount\">" + Money(detail.Monto) + "</td></tr>");
            }
            html.Append("</tbody></table><table class=\"totals\"><tr><td>MONTO TOTAL</td><td>" + Money(total) + "</td></tr><tr><td>SALDO CAJA CHICA</td><td>" + Money(balance) + "</td></tr></table>");
            html.Append("<div class=\"signatures\"><div class=\"signature\"><b>Responsables</b><br><br>" + Html(data.Header.Responsable) + "<br><span class=\"small\">" + Html(data.Header.Puesto) + "</span></div><div class=\"signature\"><b>Revisión</b><br><br>" + Html(data.Header.Revision) + "<br><span class=\"small\">" + Html(data.Header.PuestoRevision) + "</span></div></div>");
            html.Append("</div></body></html>");
            File.WriteAllText(htmlPath, html.ToString(), Encoding.UTF8);
        }

        private string FindLogo()
        {
            var reportDirectory = Path.GetDirectoryName(_options.Report);
            var appDirectory = reportDirectory == null ? null : Directory.GetParent(reportDirectory).FullName;
            if (appDirectory == null) return null;
            var photos = Path.Combine(appDirectory, "Fotos");
            if (!Directory.Exists(photos)) return null;
            var candidates = Directory.GetFiles(photos, "Principal*.*");
            foreach (var candidate in candidates)
            {
                var extension = Path.GetExtension(candidate);
                if (String.Equals(extension, ".jpg", StringComparison.OrdinalIgnoreCase) || String.Equals(extension, ".jpeg", StringComparison.OrdinalIgnoreCase) || String.Equals(extension, ".png", StringComparison.OrdinalIgnoreCase))
                {
                    var bytes = File.ReadAllBytes(candidate);
                    var mime = String.Equals(extension, ".png", StringComparison.OrdinalIgnoreCase) ? "image/png" : "image/jpeg";
                    return "data:" + mime + ";base64," + Convert.ToBase64String(bytes);
                }
            }
            return null;
        }

        private static string Html(string value)
        {
            if (value == null) return "";
            return value.Replace("&", "&amp;").Replace("<", "&lt;").Replace(">", "&gt;").Replace("\"", "&quot;");
        }

        private static string FormatDate(DateTime? value)
        {
            return value.HasValue ? value.Value.ToString("dd/MM/yyyy", CultureInfo.InvariantCulture) : "";
        }

        private static string Money(decimal value)
        {
            return value.ToString("N0", CultureInfo.InvariantCulture);
        }

        private static void RenderHtmlToPdf(string htmlPath, string outputPath, string tempRoot)
        {
            var chrome = FindBrowser();
            if (chrome == null) throw new FileNotFoundException("No se encontró Google Chrome ni Microsoft Edge para generar el PDF");

            var profile = Path.Combine(tempRoot, "chrome-profile");
            var arguments = "--headless --disable-gpu --disable-extensions --no-first-run --no-default-browser-check "
                + "--allow-file-access-from-files --no-pdf-header-footer "
                + "--user-data-dir=" + Quote(profile) + " "
                + "--print-to-pdf=" + Quote(outputPath) + " "
                + Quote(new Uri(htmlPath).AbsoluteUri);
            var start = new ProcessStartInfo
            {
                FileName = chrome,
                Arguments = arguments,
                CreateNoWindow = true,
                UseShellExecute = false,
                WindowStyle = ProcessWindowStyle.Hidden,
                WorkingDirectory = tempRoot
            };
            using (var process = Process.Start(start))
            {
                if (process == null) throw new InvalidOperationException("No se pudo iniciar Chrome headless");
                if (!process.WaitForExit(60000))
                {
                    try { process.Kill(); } catch { }
                    throw new TimeoutException("Chrome no terminó la conversión a PDF en 60 segundos");
                }
                if (process.ExitCode != 0) throw new InvalidOperationException("Chrome terminó con código " + process.ExitCode);
            }
            if (!File.Exists(outputPath) || new FileInfo(outputPath).Length == 0)
            {
                throw new IOException("Chrome terminó sin crear el archivo PDF");
            }
        }

        private static string FindBrowser()
        {
            var programW6432 = Environment.GetEnvironmentVariable("ProgramW6432");
            var candidates = new[]
            {
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Google", "Chrome", "Application", "chrome.exe"),
                Path.Combine(programW6432 ?? "", "Google", "Chrome", "Application", "chrome.exe"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "Google", "Chrome", "Application", "chrome.exe"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86), "Google", "Chrome", "Application", "chrome.exe"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Microsoft", "Edge", "Application", "msedge.exe"),
                Path.Combine(programW6432 ?? "", "Microsoft", "Edge", "Application", "msedge.exe"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "Microsoft", "Edge", "Application", "msedge.exe"),
                Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFilesX86), "Microsoft", "Edge", "Application", "msedge.exe")
            };
            foreach (var candidate in candidates)
            {
                if (File.Exists(candidate)) return candidate;
            }
            return null;
        }

        private static string Quote(string value)
        {
            return "\"" + value.Replace("\"", "\\\"") + "\"";
        }

        private static void TryDeleteDirectory(string path)
        {
            try
            {
                if (Directory.Exists(path)) Directory.Delete(path, true);
            }
            catch
            {
                // La limpieza temporal no debe ocultar el resultado de la exportación.
            }
        }

        private void CloseWithoutDisposing(object sender, FormClosingEventArgs args)
        {
            args.Cancel = true;
            _form.Hide();
            Application.ExitThread();
        }

        private void Cleanup()
        {
            _stage = "com_cleanup";
            if (_viewer != null)
            {
                _viewer.ReleaseActiveX();
                _viewer.Dispose();
                _viewer = null;
            }
            if (_form != null)
            {
                _form.Dispose();
                _form = null;
            }
            ReleaseCom(ref _report);
            ReleaseCom(ref _application);
            GC.Collect();
            GC.WaitForPendingFinalizers();
            Protocol.Stage(_stage, true, null, null, null,
                null);
        }

        private static void ReleaseCom(ref object value)
        {
            if (value == null) return;
            if (Marshal.IsComObject(value)) Marshal.FinalReleaseComObject(value);
            value = null;
        }

        private static Exception Unwrap(Exception error)
        {
            while (error is TargetInvocationException && error.InnerException != null) error = error.InnerException;
            return error;
        }

        private static string ErrorCode(Exception error)
        {
            var com = error as COMException;
            return com == null ? "automation_error" : Protocol.FormatHResult(com.ErrorCode);
        }
    }

    internal sealed class CajaChicaData
    {
        internal CajaChicaHeader Header = new CajaChicaHeader();
        internal List<CajaChicaDetail> Details = new List<CajaChicaDetail>();
    }

    internal sealed class CajaChicaHeader
    {
        internal int Id;
        internal string Caja;
        internal string Cheque;
        internal DateTime? FechaInicio;
        internal DateTime? FechaCierre;
        internal decimal MontoEstablecido;
        internal string PeriodoAnio;
        internal string Responsable;
        internal string Puesto;
        internal string Revision;
        internal string PuestoRevision;
    }

    internal sealed class CajaChicaDetail
    {
        internal DateTime? Fecha;
        internal string Factura;
        internal string Codigo;
        internal string Proveedor;
        internal string Descripcion;
        internal decimal Monto;
    }

    internal sealed class CrystalViewerControl : AxHost
    {
        internal CrystalViewerControl(string clsid) : base(clsid) { }

        internal void SetReportSource(object report)
        {
            InvokeMember("ReportSource", BindingFlags.SetProperty, new object[] { report });
        }

        internal void ViewReport()
        {
            InvokeMember("ViewReport", BindingFlags.InvokeMethod, null);
        }

        internal void ReleaseActiveX()
        {
            var ocx = GetOcx();
            if (ocx != null && Marshal.IsComObject(ocx)) Marshal.FinalReleaseComObject(ocx);
        }

        private void InvokeMember(string name, BindingFlags flags, object[] args)
        {
            var ocx = GetOcx();
            ocx.GetType().InvokeMember(name, flags | BindingFlags.Public | BindingFlags.Instance, null, ocx, args);
        }
    }
}
