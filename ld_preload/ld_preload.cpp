#include <dirent.h>
#include <dlfcn.h>
#include <errno.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/un.h>
#include <unistd.h>

#include <functional>
#include <iostream>
#include <mutex>
#include <stdexcept>
#include <string>
#include <string_view>
#include <thread>

namespace sigmaos {
namespace ld_preload {

class PathInterceptor final {
  private:
    int socket_fd_ = 0;
    bool initialized_ = false;
    std::mutex init_mutex_;

    void Initialize() {
        if (initialized_)
            return;

        std::lock_guard<std::mutex> lock(init_mutex_);

        const char *sfd_str = getenv("SIGMA_PYPROXY_FD");
        if (!sfd_str) {
            throw std::runtime_error(
                "SIGMA_PYPROXY_FD environment variable not set");
        }

        socket_fd_ = std::atoi(sfd_str);
        if (socket_fd_ <= 0) {
            throw std::runtime_error("Invalid socket file descriptor");
        }

        // Initialize proxy communication
        if (write(socket_fd_, "pb\n", 3) != 3) {
            throw std::runtime_error("Failed to write to proxy socket");
        }

        WaitForProxyResponse();
        initialized_ = true;
    }

    void WaitForProxyResponse() {
        char response;
        while (read(socket_fd_, &response, 1) == 1 && response != 'd') {
            // Wait for 'd' response
        }
    }

    void SendPathToProxy(std::string_view path) {
        std::string message = "pf";
        message += path;
        message += '\n';

        if (write(socket_fd_, message.c_str(), message.length()) !=
            static_cast<ssize_t>(message.length())) {
            throw std::runtime_error("Failed to send path to proxy");
        }

        WaitForProxyResponse();
    }

  public:
    static PathInterceptor &Instance() {
        static PathInterceptor instance;

        instance.Initialize();
        return instance;
    }

    std::string TransformPath(const char *filename) {
        std::cout << "TransformPath called with: "
                  << (filename ? filename : "null") << std::endl;

        const std::string OLD_PREFIX = "/~~";
        const std::string NEW_PREFIX = "/tmp/python";
        const std::string LIB_PREFIX = "/tmp/python/Lib";
        const std::string SUPERLIB_PATH = "/tmp/python/superlib";

        if (!filename)
            return "";

        std::string_view path(filename);

        if (path.starts_with(NEW_PREFIX)) {
            return std::string(path);
        }

        // Python's initial call to obtain all present libraries
        if (path == "/~~/Lib" || path == "/tmp/python/Lib") {
            return SUPERLIB_PATH;
        }

        // Check for /~~ prefix (Python files)
        // and replace with /tmp/python
        if (path == OLD_PREFIX) {
            return NEW_PREFIX;
        } else if (path.starts_with(OLD_PREFIX + "/")) {
            auto without_prefix = std::string(path.substr(OLD_PREFIX.length()));
            SendPathToProxy(without_prefix);
            return NEW_PREFIX + without_prefix;
        }

        // Check for /tmp/python/Lib prefix
        if (path == LIB_PREFIX) {
            return "/Lib";
        } else if (path.starts_with(LIB_PREFIX + "/")) {
            // Just remove the /tmp/python part, but keep /Lib
            auto without_prefix = std::string(path.substr(NEW_PREFIX.length()));
            SendPathToProxy(without_prefix);
            return std::string(filename);
        }

        return std::string(filename);
    }
};

// Template for function interception to reduce code duplication
template <typename FuncType> class FunctionInterceptor final {
  private:
    FuncType original_func_ = nullptr;
    const char *func_name_;

  public:
    explicit FunctionInterceptor(const char *func_name)
        : func_name_(func_name) {}

    FuncType GetOriginal() {
        if (!original_func_) {
            original_func_ =
                reinterpret_cast<FuncType>(dlsym(RTLD_NEXT, func_name_));
            if (!original_func_) {
                throw std::runtime_error(
                    std::string("Failed to load original function: ") +
                    func_name_);
            }
        }
        return original_func_;
    }
};

} // namespace ld_preload
} // namespace sigmaos

// C interface functions
extern "C" {

using namespace sigmaos::ld_preload;

int stat(const char *path, struct stat *buf) {
    static FunctionInterceptor<int (*)(const char *, struct stat *)>
        interceptor("stat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(path);
    return interceptor.GetOriginal()(transformed_path.c_str(), buf);
}

int open(const char *filename, int flags, mode_t mode) {
    static FunctionInterceptor<int (*)(const char *, int, mode_t)> interceptor(
        "open");

    auto transformed_path = PathInterceptor::Instance().TransformPath(filename);
    return interceptor.GetOriginal()(transformed_path.c_str(), flags, mode);
}

FILE *fopen(const char *filename, const char *mode) {
    static FunctionInterceptor<FILE *(*)(const char *, const char *)>
        interceptor("fopen");

    auto transformed_path = PathInterceptor::Instance().TransformPath(filename);
    return interceptor.GetOriginal()(transformed_path.c_str(), mode);
}

int openat(int dirfd, const char *pathname, int flags, mode_t mode) {
    static FunctionInterceptor<int (*)(int, const char *, int, mode_t)>
        interceptor("openat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(dirfd, transformed_path.c_str(), flags,
                                     mode);
}

DIR *opendir(const char *name) {
    static FunctionInterceptor<DIR *(*)(const char *)> interceptor("opendir");

    auto transformed_path = PathInterceptor::Instance().TransformPath(name);
    return interceptor.GetOriginal()(transformed_path.c_str());
}

int newfstatat(int dirfd, const char *pathname, struct stat *statbuf,
               int flags) {
    static FunctionInterceptor<int (*)(int, const char *, struct stat *, int)>
        interceptor("newfstatat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(dirfd, transformed_path.c_str(), statbuf,
                                     flags);
}

int fstatat(int dirfd, const char *pathname, struct stat *statbuf, int flags) {
    static FunctionInterceptor<int (*)(int, const char *, struct stat *, int)>
        interceptor("fstatat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(dirfd, transformed_path.c_str(), statbuf,
                                     flags);
}

int lstat(const char *pathname, struct stat *statbuf) {
    static FunctionInterceptor<int (*)(const char *, struct stat *)>
        interceptor("lstat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), statbuf);
}

ssize_t readlink(const char *pathname, char *buf, size_t bufsiz) {
    static FunctionInterceptor<ssize_t (*)(const char *, char *, size_t)>
        interceptor("readlink");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), buf, bufsiz);
}

int creat(const char *pathname, mode_t mode) {
    static FunctionInterceptor<int (*)(const char *, mode_t)> interceptor(
        "creat");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), mode);
}

FILE *fopen64(const char *pathname, const char *mode) {
    static FunctionInterceptor<FILE *(*)(const char *, const char *)>
        interceptor("fopen64");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), mode);
}

int open64(const char *pathname, int flags, mode_t mode) {
    static FunctionInterceptor<int (*)(const char *, int, mode_t)> interceptor(
        "open64");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), flags, mode);
}

int stat64(const char *pathname, struct stat64 *statbuf) {
    static FunctionInterceptor<int (*)(const char *, struct stat64 *)>
        interceptor("stat64");

    auto transformed_path = PathInterceptor::Instance().TransformPath(pathname);
    return interceptor.GetOriginal()(transformed_path.c_str(), statbuf);
}

} // extern "C"