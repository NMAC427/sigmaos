#include <pybind11/pybind11.h>
#include <pybind11/stl.h> // Required for automatic conversion of STL containers

#include <memory>
#include <string>

#include <proc/status.h>
#include <proxy/sigmap/sigmap.h>
#include <proxy/sigmap/proto/spproxy.pb.h>

namespace py = pybind11;

// Global client object, mirroring the original design.
std::unique_ptr<sigmaos::proxy::sigmap::Clnt> clnt;

// This function initializes the client. It will be called from Python.
void init_socket() {
  clnt = std::make_unique<sigmaos::proxy::sigmap::Clnt>();
}

// Wrapper for the Stat function to return a Python object.
py::object stat_wrapper(const std::string& pn) {
    auto result = clnt->Stat(pn);
    if (result.has_value()) {
        // pybind11 will automatically convert the TstatProto object to a Python object.
        return py::cast(*result.value());
    }
    // Return None if the stat fails.
    return py::none();
}

// Wrapper for GetFile to return a Python string (or bytes).
py::object get_file_wrapper(const std::string& pn) {
    auto result = clnt->GetFile(pn);
    if (result.has_value()) {
        // Return the file content as a Python string.
        return py::cast(*result.value());
    }
    return py::none();
}

// A more Pythonic read implementation that returns bytes.
py::bytes read_wrapper(int fd, size_t len) {
    std::string buffer(len, '\0');
    auto result = clnt->Read(fd, &buffer);
    if (result.has_value()) {
        buffer.resize(result.value());
        return py::bytes(buffer);
    }
    return py::bytes("");
}

// A more Pythonic pread implementation that returns bytes.
py::bytes pread_wrapper(int fd, size_t len, uint64_t offset) {
    std::string buffer(len, '\0');
    sigmaos::sigmap::types::Toffset off = offset;
    auto result = clnt->Pread(fd, &buffer, off);
    if (result.has_value()) {
        buffer.resize(result.value());
        return py::bytes(buffer);
    }
    return py::bytes("");
}


PYBIND11_MODULE(_clntlib, m) {
    m.doc() = "SigmaOS Python client library";

    py::class_<TqidProto>(m, "Qid")
        .def(py::init<>())
        .def_property("type", &TqidProto::type, &TqidProto::set_type)
        .def_property("version", &TqidProto::version, &TqidProto::set_version)
        .def_property("path", &TqidProto::path, &TqidProto::set_path);

    py::class_<TstatProto>(m, "Stat")
        .def(py::init<>())
        .def_property("type", &TstatProto::type, &TstatProto::set_type)
        .def_property("dev", &TstatProto::dev, &TstatProto::set_dev)
        .def_property("qid", [](const TstatProto& p) { return p.qid(); }, [](TstatProto& p, const TqidProto& q) { *p.mutable_qid() = q; })
        .def_property("mode", &TstatProto::mode, &TstatProto::set_mode)
        .def_property("atime", &TstatProto::atime, &TstatProto::set_atime)
        .def_property("mtime", &TstatProto::mtime, &TstatProto::set_mtime)
        .def_property("length", &TstatProto::length, &TstatProto::set_length)
        .def_property("name", [](const TstatProto& p) { return p.name(); }, [](TstatProto& p, const std::string& s) { p.set_name(s); })
        .def_property("uid", [](const TstatProto& p) { return p.uid(); }, [](TstatProto& p, const std::string& s) { p.set_uid(s); })
        .def_property("gid", [](const TstatProto& p) { return p.gid(); }, [](TstatProto& p, const std::string& s) { p.set_gid(s); })
        .def_property("muid", [](const TstatProto& p) { return p.muid(); }, [](TstatProto& p, const std::string& s) { p.set_muid(s); });


    // ======== ProcClnt API ========
    m.def("init_socket", &init_socket, "Initialize the connection socket.");
    m.def("started", []() { clnt->Started(); }, "Signal that the process has started.");
    m.def("exited", [](uint32_t status, const std::string& msg) {
        sigmaos::proc::Tstatus s = static_cast<sigmaos::proc::Tstatus>(status);
        std::string msg_copy = msg;
        clnt->Exited(s, msg_copy);
    }, "Signal that the process has exited.");
    m.def("wait_evict", []() { clnt->WaitEvict(); }, "Wait for an eviction event.");


    // ============== SPProxy API Stubs =============
    m.def("close", [](int fd) { clnt->CloseFD(fd); });
    m.def("stat", &stat_wrapper);
    m.def("create", [](const std::string& pn, uint32_t perm, uint32_t mode) {
        auto result = clnt->Create(pn, perm, mode);
        return result.has_value() ? result.value() : -1;
    });
    m.def("open", [](const std::string& pn, uint32_t mode, bool wait) {
        auto result = clnt->Open(pn, mode, wait);
        return result.has_value() ? result.value() : -1;
    });
    m.def("rename", [](const std::string& src, const std::string& dst) { clnt->Rename(src, dst); });
    m.def("remove", [](const std::string& pn) { clnt->Remove(pn); });
    m.def("get_file", &get_file_wrapper);
    m.def("put_file", [](const std::string& pn, uint32_t perm, uint32_t mode, const std::string& data, uint64_t offset, uint64_t leaseID) {
        std::string d = data;
        auto result = clnt->PutFile(pn, perm, mode, &d, offset, leaseID);
        return result.has_value() ? result.value() : -1;
    });
    m.def("read", &read_wrapper);
    m.def("pread", &pread_wrapper);
    m.def("write", [](int fd, const std::string& bytes) {
        std::string b = bytes;
        auto result = clnt->Write(fd, &b);
        return result.has_value() ? result.value() : -1;
    });
    m.def("seek", [](int fd, uint64_t offset) { clnt->Seek(fd, offset); });
    m.def("clnt_id", []() {
        auto result = clnt->ClntID();
        return result.has_value() ? result.value() : (uint64_t)-1;
    });
  // TODO: everything after Seek in cpp/proxy/sigmap/sigmap.h
}