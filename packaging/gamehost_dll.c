// 游戏宿主（DLL）：由系统 rundll32 加载，再加载同目录 playcli.dll。
// 发布目录里不再放游戏 exe，唯一的 exe 是登录启动器。
#include <windows.h>

typedef int (__cdecl *RunGameFn)(char *server, char *token);

__declspec(dllexport) void __stdcall Launch(HWND hwnd, HINSTANCE hinst, LPSTR cmd, int show);

static void fail(const wchar_t *msg) {
    MessageBoxW(NULL, msg, L"九州问道", MB_ICONERROR | MB_OK);
}

static void gameDllPath(wchar_t *out, DWORD count) {
    HMODULE self = NULL;
    GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
                           GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
                       (LPCWSTR)(const void *)&Launch, &self);
    if (!self || !GetModuleFileNameW(self, out, count)) {
        out[0] = 0;
        return;
    }
    wchar_t *slash = wcsrchr(out, L'\\');
    if (!slash) {
        out[0] = 0;
        return;
    }
    wcscpy(slash + 1, L"playcli.dll");
}

__declspec(dllexport) void __stdcall Launch(HWND hwnd, HINSTANCE hinst, LPSTR cmd, int show) {
    (void)hwnd;
    (void)cmd;
    (void)show;
    FreeConsole();

    wchar_t dllPath[MAX_PATH] = {0};
    gameDllPath(dllPath, MAX_PATH);
    if (dllPath[0] == 0) {
        fail(L"找不到游戏 DLL：无法定位 gamehost.dll 所在目录。");
        return;
    }

    char server[1024] = "";
    char token[4096] = "";
    GetEnvironmentVariableA("XIUXIAN_SERVER", server, (DWORD)sizeof(server));
    GetEnvironmentVariableA("XIUXIAN_TOKEN", token, (DWORD)sizeof(token));
    if (token[0] == 0) {
        fail(L"请通过登录启动器进入游戏。");
        return;
    }

    HMODULE dll = LoadLibraryW(dllPath);
    if (!dll) {
        fail(L"找不到 game/playcli.dll。请从正式发布目录启动。");
        return;
    }
    RunGameFn run = (RunGameFn)GetProcAddress(dll, "RunGame");
    if (!run) {
        fail(L"game/playcli.dll 不是正确的游戏模块。");
        FreeLibrary(dll);
        return;
    }
    run(server, token);
    FreeLibrary(dll);
}