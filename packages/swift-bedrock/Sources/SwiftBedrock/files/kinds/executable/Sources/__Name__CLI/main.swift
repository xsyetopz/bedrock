import __Name__

let name = CommandLine.arguments.dropFirst().first ?? ""
print(greeting(for: name))
