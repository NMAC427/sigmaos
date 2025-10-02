message(STATUS "Building cached-srv-cpp proc")

add_executable(cached-srv-cpp
  main.cc
)

target_link_libraries(cached-srv-cpp PRIVATE 
  sigmaos_core
  spin_proto
  app_spin
  cossim_proto
  app_cossim
  cache_proto
  app_cache
)

target_include_directories(cached-srv-cpp PRIVATE ${CMAKE_SOURCE_DIR})