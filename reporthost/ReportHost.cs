// © Antony Monge López — Costa Rica — Céd. 604700548

using System;
using System.Collections;
using System.Collections.Generic;
using System.Diagnostics;
using System.Reflection;
using System.Runtime.InteropServices;
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
            if (o.Mode != "check" && o.Mode != "view") o.Error = "modo inválido: " + o.Mode;
            if (String.IsNullOrWhiteSpace(o.Report)) o.Error = "falta --report";
            if (String.IsNullOrWhiteSpace(o.Dsn)) o.Error = "falta --dsn";
            if (String.IsNullOrWhiteSpace(o.Database)) o.Error = "falta --database";
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
