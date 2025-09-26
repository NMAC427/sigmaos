#pragma once

#include "cpp/io/conn/conn.h"
#include "cpp/io/transport/call.h"
#include "cpp/io/transport/internal/callmap.h"
#include "cpp/serr/serr.h"
#include <sys/un.h>
#include <unistd.h>
#include "cpp/util/log/log.h"

#include <expected>
#include <memory>

namespace sigmaos {
namespace io::transport {

const std::string TRANSPORT = "TRANSPORT";
const std::string TRANSPORT_ERR = "TRANSPORT" + sigmaos::util::log::ERR;

class Transport {
 public:
  Transport(std::shared_ptr<sigmaos::io::conn::Conn> conn)
      : _conn(conn), _calls() {
    log(TRANSPORT, "New transport connID: {}", conn->GetID());
  }

  ~Transport() { _conn->Close(); }

  std::expected<int, sigmaos::serr::Error> WriteCall(
      std::shared_ptr<Call> call);
  std::expected<std::shared_ptr<Call>, sigmaos::serr::Error> ReadCall();
  std::expected<int, sigmaos::serr::Error> Close();

 private:
  std::shared_ptr<sigmaos::io::conn::Conn> _conn;
  sigmaos::io::transport::internal::CallMap _calls;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;
};

};  // namespace io::transport
};  // namespace sigmaos
