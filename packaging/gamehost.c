// 游戏宿主：用原生 exe 加载 game/playcli.dll，启动器只负责登录/拉起。
#include <windows.h>

typedef int (*RunGameFn)(char *server, char *token);

static void fail(const char *msg) {
    MessageBoxA(NULL, msg, "九州问道", MB_ICONERROR | MB_OK);
}

int WINAPI WinMain(HINSTANCE hInst, HINSTANCE hPrev, LPSTR cmd, int show) {
    FreeConsole();
    char server[1024] = "";
    char token[4096] = "";
    GetEnvironmentVariableA("XIUXIAN_SERVER", server, (DWORD)sizeof(server));
    GetEnvironmentVariableA("XIUXIAN_TOKEN", token, (DWORD)sizeof(token));

    HMODULE dll = LoadLibraryW(L"playcli.dll");
    if (!dll) {
        fail("找不到 game/playcli.dll。请从正式发布目录启动。");
        return 1;
    }
    RunGameFn run = (RunGameFn)GetProcAddress(dll, "RunGame");
    if (!run) {
        fail("game/playcli.dll 不是正确的游戏模块。");
        FreeLibrary(dll);
        return 1;
    }
    int rc = run(server, token);
    FreeLibrary(dll);
    return rc;
}
