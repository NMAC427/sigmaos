#pragma once

#include "example/example_echo_server/proto/example_echo_server.pb.h"
#include "cpp/io/conn/conn.h"
#include "cpp/io/conn/tcp/tcp.h"
#include "cpp/io/demux/srv.h"
#include "cpp/io/net/srv.h"
#include "cpp/io/transport/transport.h"
#include "cpp/proxy/sigmap/sigmap.h"
#include "cpp/rpc/srv.h"
#include "cpp/serr/serr.h"
#include "cpp/sigmap/const.h"
#include "sigmap/sigmap.pb.h"
#include "cpp/util/log/log.h"

#include <expected>
#include <format>
#include <memory>
#include <vector>

namespace sigmaos {
namespace apps::echo {

const std::string ECHOSRV = "ECHOSRV";
const std::string ECHOSRV_ERR = "ECHOSRV" + sigmaos::util::log::ERR;

const std::string ECHOSRV_REALM_PN = "name/echo-srv-cpp";

class Srv {
 public:
  Srv(std::shared_ptr<sigmaos::proxy::sigmap::Clnt> sp_clnt)
      : _sp_clnt(sp_clnt) {
    log(ECHOSRV, "Starting RPC srv");
    _srv = std::make_shared<sigmaos::rpc::srv::Srv>(sp_clnt);
    log(ECHOSRV, "Started RPC srv");
    auto echo_ep = std::make_shared<sigmaos::rpc::srv::RPCEndpoint>(
        "EchoSrv.Echo", std::make_shared<EchoReq>(),
        std::make_shared<EchoRep>(),
        std::bind(&Srv::Echo, this, std::placeholders::_1,
                  std::placeholders::_2));
    _srv->ExposeRPCHandler(echo_ep);
    log(ECHOSRV, "Exposed echo RPC handler");
    {
      auto res = _srv->RegisterEP(ECHOSRV_REALM_PN);
      if (!res.has_value()) {
        log(ECHOSRV_ERR, "Error RegisterEP: {}", res.error());
        fatal("Error RegisterEP: {}", res.error().String());
      }
      log(ECHOSRV, "Registered sigmaEP");
    }
  }
  ~Srv() {}

  [[noreturn]] void Run();

 private:
  std::shared_ptr<sigmaos::proxy::sigmap::Clnt> _sp_clnt;
  std::shared_ptr<sigmaos::rpc::srv::Srv> _srv;
  // Used for logger initialization
  static bool _l;
  static bool _l_e;

  // Echo RPC handler
  std::expected<int, sigmaos::serr::Error> Echo(
      std::shared_ptr<google::protobuf::Message> preq,
      std::shared_ptr<google::protobuf::Message> prep);
};

};  // namespace apps::echo
};  // namespace sigmaos
