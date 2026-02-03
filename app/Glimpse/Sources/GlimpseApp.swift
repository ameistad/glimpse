import SwiftUI

@main
struct GlimpseApp: App {
    @StateObject private var settings = AppSettings()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(settings)
        }
        .commands {
            CommandGroup(replacing: .newItem) { }
        }

        Settings {
            SettingsView()
                .environmentObject(settings)
        }
    }
}

class AppSettings: ObservableObject {
    @AppStorage("serverURL") var serverURL: String = ""
    @AppStorage("downloadPath") var downloadPath: String = ""
    @Published var needsRefresh: Bool = false

    init() {
        if downloadPath.isEmpty {
            downloadPath = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask).first?.path ?? ""
        }
    }
}
