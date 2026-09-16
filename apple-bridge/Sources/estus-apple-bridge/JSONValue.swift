import Foundation

/// Sendable representation of an arbitrary JSON value.
enum JSONValue: Sendable, Equatable {
    case null
    case bool(Bool)
    case int(Int)
    case number(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    struct ParseError: Error, LocalizedError {
        let message: String
        var errorDescription: String? { message }
    }

    static func parse(_ data: Data) throws -> JSONValue {
        let any: Any
        do {
            any = try JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
        } catch {
            throw ParseError(message: "invalid JSON: \(error.localizedDescription)")
        }
        return JSONValue(any: any)
    }

    init(any: Any) {
        switch any {
        case is NSNull:
            self = .null
        case let n as NSNumber:
            if CFGetTypeID(n) == CFBooleanGetTypeID() {
                self = .bool(n.boolValue)
            } else if CFNumberIsFloatType(n) {
                self = .number(n.doubleValue)
            } else {
                self = .int(n.intValue)
            }
        case let s as String:
            self = .string(s)
        case let a as [Any]:
            self = .array(a.map { JSONValue(any: $0) })
        case let d as [String: Any]:
            self = .object(d.mapValues { JSONValue(any: $0) })
        default:
            self = .null
        }
    }

    var foundationValue: Any {
        switch self {
        case .null: return NSNull()
        case .bool(let b): return NSNumber(value: b)
        case .int(let i): return NSNumber(value: i)
        case .number(let d): return NSNumber(value: d)
        case .string(let s): return s
        case .array(let a): return a.map { $0.foundationValue }
        case .object(let o): return o.mapValues { $0.foundationValue }
        }
    }

    /// Compact JSON encoding.
    func serialized() -> Data {
        let data = try? JSONSerialization.data(
            withJSONObject: foundationValue,
            options: [.fragmentsAllowed, .withoutEscapingSlashes, .sortedKeys]
        )
        return data ?? Data("null".utf8)
    }

    func serializedString() -> String {
        String(decoding: serialized(), as: UTF8.self)
    }

    // MARK: accessors

    subscript(key: String) -> JSONValue? {
        if case .object(let o) = self { return o[key] }
        return nil
    }

    var stringValue: String? {
        if case .string(let s) = self { return s }
        return nil
    }

    var arrayValue: [JSONValue]? {
        if case .array(let a) = self { return a }
        return nil
    }

    var objectValue: [String: JSONValue]? {
        if case .object(let o) = self { return o }
        return nil
    }

    var intValue: Int? {
        switch self {
        case .int(let i): return i
        case .number(let d): return Int(exactly: d)
        default: return nil
        }
    }
}
