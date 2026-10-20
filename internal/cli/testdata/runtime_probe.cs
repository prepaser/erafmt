using System;
using System.IO;
using System.Reflection;
using System.Text;
using System.Linq;
using System.Globalization;
using System.Collections;
using System.Collections.Generic;
using System.Security.Cryptography;

internal static class RuntimeProbe
{
    private const BindingFlags Any = BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance | BindingFlags.Static;
    private static Assembly runtime;
    private static string runtimeDirectory;
    private static Type globalStatic;
    private static object parser;
    private static object argumentParser;

    private static Type Find(string name)
    {
        Type exact = runtime.GetType(name, false, false);
        if (exact != null)
            return exact;
        foreach (Type type in runtime.GetTypes())
        {
            if (type.Name == name || (type.FullName != null && type.FullName.EndsWith("." + name, StringComparison.Ordinal)))
                return type;
        }
        throw new InvalidOperationException("runtime type not found: " + name);
    }

    private static MethodInfo Method(Type type, string name, int count)
    {
        MethodInfo method = type.GetMethods(Any).FirstOrDefault(m => m.Name == name && m.GetParameters().Length == count);
        if (method == null)
            throw new InvalidOperationException("runtime method not found: " + type.FullName + "." + name);
        return method;
    }

    private static object New(Type type, params object[] args)
    {
        return Activator.CreateInstance(type, Any, null, args, null);
    }

    private static object NewWithNullConsole(Type type)
    {
        Type consoleType = Find("EmueraConsole");
        ConstructorInfo constructor = type.GetConstructor(Any, null, new[] { consoleType }, null);
        if (constructor == null)
            throw new InvalidOperationException("runtime console constructor not found: " + type.FullName);
        return constructor.Invoke(new object[] { null });
    }

    private static object Get(object target, string name)
    {
        Type type = target is Type ? (Type)target : target.GetType();
        PropertyInfo property = type.GetProperty(name, Any);
        if (property != null)
            return property.GetValue(target is Type ? null : target, null);
        FieldInfo field = type.GetField(name, Any);
        if (field != null)
            return field.GetValue(target is Type ? null : target);
        throw new InvalidOperationException("runtime member not found: " + type.FullName + "." + name);
    }

    private static void Set(string name, object value)
    {
        FieldInfo field = globalStatic.GetField(name, Any);
        if (field == null)
            throw new InvalidOperationException("runtime global not found: " + name);
        field.SetValue(null, value);
    }

    private static void Configure(string csvDirectory)
    {
        if (!csvDirectory.EndsWith(Path.DirectorySeparatorChar.ToString(), StringComparison.Ordinal) && !csvDirectory.EndsWith(Path.AltDirectorySeparatorChar.ToString(), StringComparison.Ordinal))
            csvDirectory += Path.DirectorySeparatorChar;
        Type configData = Find("ConfigData");
        object config = Get(configData, "Instance");
        Type configType = Find("Config");
        Method(configType, "SetConfig", 1).Invoke(null, new object[] { config });
        Method(configType, "SetReplace", 1).Invoke(null, new object[] { config });

        Type gameBaseType = Find("GameBase");
        object gameBase = New(gameBaseType);
        Type constantType = Find("ConstantData");
        object constant = New(constantType);
        Method(constantType, "LoadData", 3).Invoke(constant, new object[] { csvDirectory, null, false });
        Set("GameBaseData", gameBase);
        Set("ConstantData", constant);

        Type variableEvaluatorType = Find("VariableEvaluator");
        object variableEvaluator = New(variableEvaluatorType, gameBase, constant);
        Set("VEvaluator", variableEvaluator);
        object variableData = Get(variableEvaluator, "VariableData");
        Set("VariableData", variableData);

        Type processType = Find("Process");
        object process = NewWithNullConsole(processType);
        Type processStateType = Find("ProcessState");
        object processState = NewWithNullConsole(processStateType);
        FieldInfo processStateField = processType.GetField("state", Any);
        if (processStateField == null)
            throw new InvalidOperationException("runtime process state field not found");
        processStateField.SetValue(process, processState);
        Set("Process", process);

        Type identifierType = Find("IdentifierDictionary");
        object identifier = New(identifierType, variableData);
        Set("IdentifierDictionary", identifier);

        InvokeOptional("StrForm", "Initialize", 0);
        InvokeOptional("VariableParser", "Initialize", 0);

        Type mediatorType = Find("ExpressionMediator");
        object mediator = New(mediatorType, null, variableEvaluator, null);
        Set("EMediator", mediator);
        Type labelsType = Find("LabelDictionary");
        Set("LabelDictionary", New(labelsType));

        parser = Find("LogicalLineParser");
        argumentParser = Find("ArgumentParser");
    }

    private static void InvokeOptional(string typeName, string methodName, int count)
    {
        Type type = null;
        try { type = Find(typeName); }
        catch (InvalidOperationException) { return; }
        Method(type, methodName, count).Invoke(null, new object[count]);
    }

    private static string StringValue(object value)
    {
        return value == null ? "" : value.ToString();
    }

    private static string Json(string value)
    {
        if (value == null)
            return "null";
        StringBuilder result = new StringBuilder(value.Length + 2);
        result.Append('"');
        foreach (char ch in value)
        {
            switch (ch)
            {
                case '\\': result.Append("\\\\"); break;
                case '"': result.Append("\\\""); break;
                case '\r': result.Append("\\r"); break;
                case '\n': result.Append("\\n"); break;
                case '\t': result.Append("\\t"); break;
                default:
                    if (ch < 0x20)
                        result.Append("\\u" + ((int)ch).ToString("x4"));
                    else
                        result.Append(ch);
                    break;
            }
        }
        result.Append('"');
        return result.ToString();
    }

    private static string ErrorKind(string error)
    {
        if (String.IsNullOrEmpty(error))
            return "";
        if (error.IndexOf("numeric", StringComparison.OrdinalIgnoreCase) >= 0 || error.IndexOf("数値", StringComparison.Ordinal) >= 0)
            return "numeric";
        if (error.IndexOf("type", StringComparison.OrdinalIgnoreCase) >= 0 || error.IndexOf("型", StringComparison.Ordinal) >= 0)
            return "type";
        return "syntax";
    }

    private sealed class EvidenceWriter
    {
        private const int MaxDepth = 24, MaxItems = 64, MaxNodes = 4096, MaxChars = 65536;
        private readonly StringBuilder text = new StringBuilder();
        private readonly List<object> ancestors = new List<object>();
        private int nodes;

        private void Append(string value)
        {
            string prefix = value.Length.ToString(CultureInfo.InvariantCulture) + ":";
            if (value.Length > MaxChars - text.Length - prefix.Length)
                throw new InvalidOperationException("runtime evidence character limit exceeded");
            text.Append(prefix).Append(value);
        }

        private static bool IsVariableToken(Type type)
        {
            for (; type != null; type = type.BaseType)
                if (type.Name == "VariableToken")
                    return true;
            return false;
        }

        private void Write(object value, int depth)
        {
            if (++nodes > MaxNodes || depth > MaxDepth)
                throw new InvalidOperationException("runtime evidence traversal limit exceeded");
            if (value == null)
            {
                Append("null");
                return;
            }
            Type type = value.GetType();
            Type representedType = value as Type;
            if (representedType != null)
            {
                Append("type");
                Append(representedType.FullName);
                return;
            }
            Append(type.FullName);
            if (type.IsPrimitive || type.IsEnum || value is string || value is decimal)
            {
                if (value is double)
                    Append(((double)value).ToString("R", CultureInfo.InvariantCulture));
                else if (value is float)
                    Append(((float)value).ToString("R", CultureInfo.InvariantCulture));
                else
                    Append(Convert.ToString(value, CultureInfo.InvariantCulture));
                return;
            }
            if (ancestors.Any(parent => Object.ReferenceEquals(parent, value)))
                throw new InvalidOperationException("runtime evidence contains a cycle");
            ancestors.Add(value);
            try
            {
                IEnumerable items = value as IEnumerable;
                if (items != null)
                {
                    Append("items");
                    int count = 0;
                    foreach (object item in items)
                    {
                        if (++count > MaxItems)
                            throw new InvalidOperationException("runtime evidence item limit exceeded");
                        Write(item, depth + 1);
                    }
                    Append("end");
                    return;
                }
                if (IsVariableToken(type))
                {
                    // Compare identity without traversing runtime variable storage.
                    foreach (string name in new[] { "Name", "Code", "VarCodeInt", "VariableType" })
                    {
                        Append(name);
                        Write(Get(value, name), depth + 1);
                    }
                    return;
                }
                if (type.Assembly != runtime)
                    throw new InvalidOperationException("unsupported runtime evidence type: " + type.FullName);
                for (Type owner = type; owner != null && owner != typeof(object); owner = owner.BaseType)
                {
                    foreach (FieldInfo field in owner.GetFields(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance | BindingFlags.DeclaredOnly).OrderBy(f => f.Name, StringComparer.Ordinal))
                    {
                        Append(owner.FullName + "." + field.Name);
                        Write(field.GetValue(value), depth + 1);
                    }
                }
                Append("end");
            }
            finally
            {
                ancestors.RemoveAt(ancestors.Count - 1);
            }
        }

        public string Fingerprint(object value)
        {
            Write(value, 0);
            using (SHA256 sha = SHA256.Create())
            {
                string serialized = text.ToString();
                byte[] codeUnits = new byte[serialized.Length * 2];
                for (int i = 0; i < serialized.Length; i++)
                {
                    char codeUnit = serialized[i];
                    codeUnits[2 * i] = (byte)codeUnit;
                    codeUnits[2 * i + 1] = (byte)(codeUnit >> 8);
                }
                return "sha256:" + Convert.ToBase64String(sha.ComputeHash(codeUnits));
            }
        }
    }

    private static string Evidence(object argument)
    {
        return argument == null ? "" : new EvidenceWriter().Fingerprint(argument);
    }

    private static string Probe(string source)
    {
        MethodInfo parse = Method((Type)parser, "ParseLine", 2);
        object line = parse.Invoke(null, new object[] { source, null });
        if (line == null)
            return "{\"accepted\":false,\"error\":\"null line\"}";
        bool error = Convert.ToBoolean(Get(line, "IsError"));
        object function = null;
        object argument = null;
        if (!error)
        {
            try
            {
                function = Get(line, "Function");
                Method(argumentParser is Type ? (Type)argumentParser : argumentParser.GetType(), "SetArgumentTo", 1).Invoke(null, new[] { line });
                error = Convert.ToBoolean(Get(line, "IsError"));
                argument = Get(line, "Argument");
            }
            catch (InvalidOperationException)
            {
                throw;
            }
        }
        string functionName = function == null ? "" : StringValue(Get(function, "Name"));
        string argumentName = argument == null ? "" : argument.GetType().FullName;
        string errorMessage = error ? StringValue(Get(line, "ErrMes")) : "";
        return "{\"accepted\":" + (!error ? "true" : "false") + ",\"function\":" + Json(functionName) + ",\"argument\":" + Json(argumentName) + ",\"evidence\":" + Json(Evidence(argument)) + ",\"errorKind\":" + Json(ErrorKind(errorMessage)) + ",\"error\":" + Json(errorMessage) + "}";
    }

    public static int Main(string[] args)
    {
        try
        {
            if (args.Length < 2)
                throw new InvalidOperationException("usage: runtime_probe assembly csv-directory");
#if NETCOREAPP
            Encoding.RegisterProvider(CodePagesEncodingProvider.Instance);
            CultureInfo.CurrentCulture = CultureInfo.InvariantCulture;
            CultureInfo.DefaultThreadCurrentCulture = CultureInfo.InvariantCulture;
#endif
            runtimeDirectory = Path.GetDirectoryName(Path.GetFullPath(args[0]));
            AppDomain.CurrentDomain.AssemblyResolve += ResolveRuntimeDependency;
            runtime = Assembly.LoadFrom(args[0]);
            globalStatic = Find("GlobalStatic");
            Configure(args[1]);
            string encoded;
            while ((encoded = Console.ReadLine()) != null)
            {
                string source = Encoding.UTF8.GetString(Convert.FromBase64String(encoded));
                Console.WriteLine(Probe(source));
            }
            return 0;
        }
        catch (Exception error)
        {
            Console.Error.WriteLine(error.ToString());
            return 3;
        }
    }

    private static Assembly ResolveRuntimeDependency(object sender, ResolveEventArgs args)
    {
        string name = new AssemblyName(args.Name).Name + ".dll";
        string path = Path.Combine(runtimeDirectory, name);
        return File.Exists(path) ? Assembly.LoadFrom(path) : null;
    }
}
